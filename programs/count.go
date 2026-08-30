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
		0xa9, 0x00,			// lda #$00		; A = 0
		0xd8, 				// cld			; clear decimal flag
		0x85, 0x00,			// sta $00		; store a in address 0x0000, will be visible in data line
        0x18,				// clc			; clear carry flag
		0x69, 0x01,			// adc #$01		; increment A by 1
		0x4c, 0x03, 0x80,	// jmp 0x8003	; jump to sta instruction
	}

	// Fill image with nops
	for len(rom) < eepromSize {
		rom = append(rom, 0xea)
	}

	// Set reset vector to start of eeprom
	rom[0x7ffc] = 0x00
	rom[0x7ffd] = 0x80

	os.WriteFile("./count.bin", rom, 0660)
}
