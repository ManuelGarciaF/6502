package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"go.bug.st/serial"
)

var serialMode = serial.Mode{
	BaudRate:          1000000,
	InitialStatusBits: &serial.ModemOutputBits{DTR: false, RTS: false},
}

func main() {
	if err := root(os.Args[1:]); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func root(args []string) error {
	if len(args) < 1 {
		return errors.New("usage: flasher <dump|flash> [options]")
	}

	switch args[0] {
	case "dump":
		return runDump(args[1:])
	default:
		return fmt.Errorf("unknown subcommand: %s", args[0])
	}
}


func runDump(args []string) error {
	// Flags
	fs := flag.NewFlagSet("dump", flag.ExitOnError)
	outPath := fs.String("o", "./dump.bin", "Path to write the dump to")
	portName := fs.String("p", "/dev/ttyUSB0", "Serial port to connect to the arduino")
	if err := fs.Parse(args); err != nil {
		return err
	}

	port, err := openPort(*portName)
	if err != nil {
		return err
	}
	defer port.Close()

	device := NewDevice(port)

	fmt.Println("Starting dump...")
	dump, err := device.Dump()
	if err != nil {
		return err
	}

	// Save dump
	fmt.Printf("Saving to %s...\n", *outPath)
	if err := os.WriteFile(*outPath, dump, 0644); err != nil {
		return err
	}

	return nil
}

func openPort(name string) (serial.Port, error) {
	port, err := serial.Open(name, &serialMode)
	if err != nil {
		return nil, err
	}
	if err := port.ResetInputBuffer(); err != nil {
		port.Close()
		return nil, err
	}
	if err := port.SetReadTimeout(2 * time.Second); err != nil {
		port.Close()
		return nil, err
	}
	return port, nil
}
