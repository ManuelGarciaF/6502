package main

import (
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/ManuelGarciaF/6502/eeprom-flasher-host/protocol"
)

/*
 * The two operations implemented in the protocol are:
 *
 * Read:  -> Header[MagicHost,   CodeRead,  StartAddress, 1           ] Payload[BytesToRead] Checksum
 *        <- Header[MagicDevice, CodeACK,   StartAddress, BytesRead   ]	Payload[data]        Checksum
 *
 * Write: -> Header[MagicHost,   CodeWrite, StartAddress, BytesToWrite] Payload[data]        Checksum
 *        <- Header[MagicDevice, CodeACK,   StartAddress, 0           ]                      Checksum
 *
 *   Payload is limited to 64 bytes in length for both reads and writes.
 *   Writes should be confined to a single EEPROM page, therefore BytesToWrite <= 64, and StartAddress must be
 *   page aligned (lowest 6 bits must be 0).
 */

const (
	eepromSize = 0x8000 // 32k bytes
	pageSize   = 64     // Size of a page in bytes.

)

type Device struct {
	r io.Reader
	w io.Writer
}

// Wraps a serial.Port so it throws an error when reading times out.
type timeoutReader struct{ r io.Reader }

func (t timeoutReader) Read(p []byte) (int, error) {
	n, err := t.r.Read(p)
	if n == 0 && err == nil && len(p) > 0 { // Timed out reads return 0, nil
		return 0, errors.New("exceeded deadline when reading from serial")
	}
	return n, err
}

func NewDevice(port io.ReadWriter) *Device {
	return &Device{r: timeoutReader{r: port}, w: port}
}

// Sends a request to the arduino and receives a response
func (d *Device) roundTrip(req *protocol.Frame) (*protocol.Frame, error) {

	// Send request
	if _, err := d.w.Write(req.Encode()); err != nil {
		return nil, fmt.Errorf("writing request for %#04x: %w", req.Address, err)
	}

	// Read a response
	res, err := protocol.Decode(d.r)
	if err != nil {
		return nil, fmt.Errorf("reading response for %#04x: %w", req.Address, err)
	}

	// Validate response
	if req.Address != res.Address {
		return nil, fmt.Errorf("adress mismatch: sent %#04x, received %#04x", req.Address, res.Address)
	}
	if res.Code != protocol.CodeACK {
		return nil, fmt.Errorf("device returned code %#02x at %#04x", res.Code, req.Address)
	}

	return res, nil
}

func (d *Device) Dump() ([]byte, error) {

	dump := make([]byte, 0, eepromSize)

	for addr := uint16(0x0000); addr < eepromSize; addr += pageSize {

		if addr%0x0200 == 0 {
			fmt.Printf("\rDumping... %3d%%", (int(addr)*100)/eepromSize)
		}

		res, err := d.roundTrip(newReadReq(addr, pageSize))
		if err != nil {
			return nil, err
		}

		if len(res.Payload) != pageSize {
			return nil, fmt.Errorf(
				"received payload has different size as req. addr: %#04x, expected: %#02x, got: %#02x",
				addr, pageSize, len(res.Payload),
			)
		}

		dump = append(dump, res.Payload...)

	}
	fmt.Println()

	fmt.Println("Done.")

	if len(dump) != eepromSize {
		return nil, fmt.Errorf("dump does not have the right size: %#04x", len(dump))
	}

	return dump, nil

}

func (d *Device) Flash(data []byte) error {

	if len(data) == 0 {
		return errors.New("image is empty")
	}
	if len(data) > eepromSize {
		return fmt.Errorf("image is %d bytes, EEPROM holds %d", len(data), eepromSize)
	}

	addr := uint16(0x0000)
	for page := range slices.Chunk(data, pageSize) {

		if addr%0x0200 == 0 {
			fmt.Printf("\rFlashing... %3d%%", (int(addr)*100)/len(data))
		}

		if _, err := d.roundTrip(newWriteReq(addr, page)); err != nil {
			return err
		}

		addr += uint16(len(page))
	}
	fmt.Println()

	return nil
}

// Read back the data on the rom and compare
func (d *Device) Verify(data []byte) error {

	addr := uint16(0x0000)
	for page := range slices.Chunk(data, pageSize) {

		if addr%0x0200 == 0 {
			fmt.Printf("\rVerifying... %3d%%", (int(addr)*100)/len(data))
		}

		res, err := d.roundTrip(newReadReq(addr, uint8(len(page))))
		if err != nil {
			return err
		}

		if len(res.Payload) != len(page) {
			return fmt.Errorf("read %d bytes at %#04x, want %d", len(res.Payload), addr, len(page))
		}

		for i := range res.Payload {
			if res.Payload[i] != data[int(addr)+i] {
				return fmt.Errorf("verify failed at %#04x: wrote %#02x, read %#02x",
					int(addr)+i, data[int(addr)+i], res.Payload[i])
			}
		}

		addr += uint16(len(page))
	}
	fmt.Println()

	return nil
}

func newReadReq(addr uint16, bytesToRead uint8) *protocol.Frame {
	return &protocol.Frame{
		Code:    protocol.CodeRead,
		Address: addr,
		Payload: []byte{byte(bytesToRead)},
	}
}

func newWriteReq(addr uint16, data []byte) *protocol.Frame {
	return &protocol.Frame{
		Code:    protocol.CodeWrite,
		Address: addr,
		Payload: data,
	}
}
