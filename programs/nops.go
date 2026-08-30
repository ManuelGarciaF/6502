///usr/bin/env go run "$0" "$@"; exit $?

//go:build ignore

package main

import (
	"os"
)

const eepromSize = 0x8000

func main() {
	bytes := make([]byte, eepromSize)
	for i := range eepromSize {
		bytes[i] = 0xea // nop
	}
	os.WriteFile("./nops.bin", bytes, 0660)
}
