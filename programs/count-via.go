///usr/bin/env go run "$0" "$@"; exit $?

//go:build ignore

package main

import (
	"os"
)

const eepromSize = 0x8000

func main() {

	// Instructions go here
	rom := []byte{
		0xa9, 0xff,			// lda #$ff       ; set DDRB to output
		0x8d, 0x02, 0x60,	// sta $6002

		0xd8,               // cld
		0x18,               // clc
		
		0xa9, 0x00,			// lda #$00       

		// loop:
		0x8d, 0x00, 0x60,	// sta $6000
		0x69, 0x01,         // adc #$01
		0x4c, 0x09, 0x80,   // jmp $8009      ; jump to loop
	}

	// Fill image with nops (0xea)
	for len(rom) < eepromSize {
		rom = append(rom, 0xea)
	}

	// Set reset vector to start of eeprom
	rom[0x7ffc] = 0x00
	rom[0x7ffd] = 0x80

	os.WriteFile("./count-via.bin", rom, 0660)
}
