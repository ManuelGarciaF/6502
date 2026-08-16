package main

import (
	"fmt"
	"io"

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
 *   Writes should be confined to a single EEPROM page, therefore BytesToWrite <= 64, and StartAddress must be
 *   page aligned (lowest 6 bits must be 0)
 */

const (
	eepromSize    = 0x8000 // 32k bytes
	readBlockSize = 128    // How many bytes to read per read request
)

type Device struct {
	port io.ReadWriter
}

func NewDevice(port io.ReadWriter) *Device { return &Device{port: port} }

// Sends a request to the arduino and receives a response
func (d *Device) roundTrip(req *protocol.Frame) (*protocol.Frame, error) {

	// Send request
	if _, err := d.port.Write(req.Encode()); err != nil {
		return nil, fmt.Errorf("writing request for %#04x: %w", req.Address, err)
	}

	// Read a response
	res, err := protocol.Decode(d.port)
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

	for addr := uint16(0x0000); addr < 0x8000; addr += readBlockSize {

		res, err := d.roundTrip(newReadReq(addr, readBlockSize))
		if err != nil {
			return nil, err
		}

		if len(res.Payload) != readBlockSize {
			return nil, fmt.Errorf(
				"received payload has different size as req. addr: %#04x, expected: %#02x, got: %#02x",
				addr, readBlockSize, len(res.Payload),
			)
		}

		dump = append(dump, res.Payload...)

	}

	if len(dump) != eepromSize {
		return nil, fmt.Errorf("dump does not have the right size: %#04x", len(dump))
	}

	return dump, nil

}

func newReadReq(addr uint16, bytesToRead uint8) *protocol.Frame {
	return &protocol.Frame{
		Code:    protocol.CodeRead,
		Address: addr,
		Payload: []byte{byte(bytesToRead)},
	}
}
