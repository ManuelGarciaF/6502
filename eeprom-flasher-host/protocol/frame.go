package protocol

import (
	"encoding/binary"
	"errors"
	"io"

	assert "github.com/ManuelGarciaF/6502/eeprom-flasher-host/utils"
)

// Serial Communication Protocol Header
// ┌────────────────┬──────────┬─────────────┬────────────┐
// │ Magic Byte (1) │ Code (1) │ Address (2) │ Length (1) │
// └────────────────┴──────────┴─────────────┴────────────┘
// Payload is at most 255 bytes long, includes header field for starting address.
// Ends with a 1 byte XOR checksum of the message.

const (
	MagicHost   byte = 0x42
	MagicDevice byte = 0x24

	headerLen  = 5
	maxPayload = 255
)

type Code byte

const (
	// Host -> Device
	CodeRead  Code = 0x01
	CodeWrite Code = 0x02

	// Device -> Host
	CodeACK Code = 0x81
	CodeNAK Code = 0x82
)

type Frame struct {
	Code    Code
	Address uint16
	Payload []byte
}

func (f *Frame) Encode() []byte { return f.encode(MagicHost) }

func (f *Frame) encode(magic byte) []byte {
	assert.LessThanOrEqual(len(f.Payload), maxPayload, "Payload must be smaller than 256 bytes")

	// header+payload+checksum
	buf := make([]byte, headerLen+len(f.Payload)+1)

	buf[0] = magic
	buf[1] = byte(f.Code)
	binary.LittleEndian.PutUint16(buf[2:4], f.Address)
	buf[4] = byte(len(f.Payload))
	copy(buf[headerLen:], f.Payload)
	buf[len(buf)-1] = checksum(buf[:len(buf)-1])

	return buf
}

var (
	ErrBadMagic        = errors.New("magic byte is incorrect")
	ErrInvalidChecksum = errors.New("checksums do not match")
)

func Decode(r io.Reader) (*Frame, error) { return decode(r, MagicDevice) }

func decode(r io.Reader, magic byte) (*Frame, error) {
	// Read a header from r
	header := make([]byte, headerLen)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}

	// Check the first byte is the correct magic byte
	if header[0] != magic {
		return nil, ErrBadMagic
	}

	f := &Frame{
		Code:    Code(header[1]),
		Address: binary.LittleEndian.Uint16(header[2:4]),
		Payload: nil,
	}

	// Read payload
	size := header[4]
	f.Payload = make([]byte, size)
	if _, err := io.ReadFull(r, f.Payload); err != nil {
		return nil, err
	}

	// Compare Checksum
	var rxChecksum [1]byte
	if _, err := io.ReadFull(r, rxChecksum[:]); err != nil {
		return nil, err
	}

	if checksum(header)^checksum(f.Payload) != rxChecksum[0] {
		return nil, ErrInvalidChecksum
	}

	return f, nil

}

// XOR Checksum
func checksum(bs []byte) byte {
	c := byte(0)
	for _, b := range bs {
		c ^= b
	}
	return c
}
