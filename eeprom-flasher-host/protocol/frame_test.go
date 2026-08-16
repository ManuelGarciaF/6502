package protocol

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

// payload returns n deterministic bytes.
func payload(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i * 7)
	}
	return b
}

// TestEncodeGolden pins the exact wire bytes for a known frame. The C++
// firmware must produce these same bytes; nothing else checks that agreement.
func TestEncodeGolden(t *testing.T) {
	f := &Frame{Code: CodeRead, Address: 0x1234, Payload: []byte{0xAA, 0xBB}}

	want := []byte{
		MagicHost,  // magic
		0x01,       // code
		0x34, 0x12, // address, little endian
		0x02,       // length
		0xAA, 0xBB, // payload
		0x76, // XOR of every preceding byte
	}

	if got := f.Encode(); !bytes.Equal(got, want) {
		t.Errorf("Encode() = % X, want % X", got, want)
	}
}

func TestRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		frame Frame
	}{
		{"empty payload", Frame{Code: CodeACK, Address: 0x0000}},
		{"single byte", Frame{Code: CodeWrite, Address: 0x7FFF, Payload: payload(1)}},
		{"one page", Frame{Code: CodeWrite, Address: 0x0040, Payload: payload(64)}},
		{"max payload", Frame{Code: CodeRead, Address: 0xBEEF, Payload: payload(maxPayload)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wire := tt.frame.encode(MagicDevice)

			got, err := decode(bytes.NewReader(wire), MagicDevice)
			if err != nil {
				t.Fatalf("decode() error = %v", err)
			}

			if got.Code != tt.frame.Code {
				t.Errorf("Code = %#x, want %#x", got.Code, tt.frame.Code)
			}
			if got.Address != tt.frame.Address {
				t.Errorf("Address = %#04x, want %#04x", got.Address, tt.frame.Address)
			}
			// bytes.Equal treats nil and empty as equal, which is what we want:
			// a nil payload encodes to length 0 and decodes to an empty slice.
			if !bytes.Equal(got.Payload, tt.frame.Payload) {
				t.Errorf("Payload = % X, want % X", got.Payload, tt.frame.Payload)
			}
		})
	}
}

func TestDecodeRejectsBadMagic(t *testing.T) {
	f := &Frame{Code: CodeACK, Address: 0x1000, Payload: payload(4)}

	// A host frame handed to the device-side decoder.
	if _, err := Decode(bytes.NewReader(f.Encode())); !errors.Is(err, ErrBadMagic) {
		t.Errorf("error = %v, want %v", err, ErrBadMagic)
	}
}

func TestDecodeRejectsBadChecksum(t *testing.T) {
	f := &Frame{Code: CodeACK, Address: 0x1000, Payload: payload(8)}

	t.Run("corrupt payload", func(t *testing.T) {
		wire := f.encode(MagicDevice)
		wire[headerLen] ^= 0x01 // Flip one bit in the first payload byte.

		if _, err := Decode(bytes.NewReader(wire)); !errors.Is(err, ErrInvalidChecksum) {
			t.Errorf("error = %v, want %v", err, ErrInvalidChecksum)
		}
	})

	t.Run("corrupt address", func(t *testing.T) {
		wire := f.encode(MagicDevice)
		wire[2] ^= 0x80

		if _, err := Decode(bytes.NewReader(wire)); !errors.Is(err, ErrInvalidChecksum) {
			t.Errorf("error = %v, want %v", err, ErrInvalidChecksum)
		}
	})

	t.Run("corrupt checksum byte", func(t *testing.T) {
		wire := f.encode(MagicDevice)
		wire[len(wire)-1] ^= 0xFF

		if _, err := Decode(bytes.NewReader(wire)); !errors.Is(err, ErrInvalidChecksum) {
			t.Errorf("error = %v, want %v", err, ErrInvalidChecksum)
		}
	})
}

// TestDecodeTruncated covers a short read at each stage. A serial port that
// times out mid-frame must produce an error, never a partially filled Frame.
func TestDecodeTruncated(t *testing.T) {
	full := (&Frame{Code: CodeACK, Address: 0x2000, Payload: payload(16)}).encode(MagicDevice)

	tests := []struct {
		name string
		n    int // bytes of the frame that arrive
		want error
	}{
		{"nothing", 0, io.EOF},
		{"partial header", 3, io.ErrUnexpectedEOF},
		{"header only", headerLen, io.EOF},
		{"partial payload", headerLen + 8, io.ErrUnexpectedEOF},
		{"missing checksum", len(full) - 1, io.EOF},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Decode(bytes.NewReader(full[:tt.n]))
			if !errors.Is(err, tt.want) {
				t.Errorf("error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestEncodePanicsOnOversizedPayload(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected a panic for a payload larger than maxPayload")
		}
	}()

	f := &Frame{Code: CodeWrite, Payload: payload(maxPayload + 1)}
	f.Encode()
}
