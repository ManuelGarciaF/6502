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
	BaudRate: 1000000,
}

func main() {
	if err := root(os.Args[1:]); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func root(args []string) error {
	if len(args) < 1 {
		return errors.New("usage: eeprom-flasher-host <dump|flash> [options]")
	}

	switch args[0] {
	case "dump":
		return runDump(args[1:])
	case "flash":
		return runFlash(args[1:])
	default:
		return fmt.Errorf("unknown subcommand: %s", args[0])
	}
}

func runDump(args []string) error {
	// Parse flags.
	fs := flag.NewFlagSet("dump", flag.ExitOnError)
	outPath := fs.String("o", "dump.bin", "Path to write the dump to")
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

	dump, err := device.Dump()
	if err != nil {
		return err
	}

	// Save dump.
	if err := os.WriteFile(*outPath, dump, 0644); err != nil {
		return err
	}

	fmt.Printf("Saved %d bytes to %s\n", len(dump), *outPath)

	return nil
}

func runFlash(args []string) error {
	// Parse flags.
	fs := flag.NewFlagSet("flash", flag.ExitOnError)
	portName := fs.String("p", "/dev/ttyUSB0", "Serial port to connect to the arduino")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.Arg(0) == "" {
		return fmt.Errorf("must supply a path to a .bin file for flashing")
	}

	image, err := os.ReadFile(fs.Arg(0))
	if err != nil {
		return err
	}

	port, err := openPort(*portName)
	if err != nil {
		return err
	}
	defer port.Close()

	device := NewDevice(port)

	if err := device.Flash(image); err != nil {
		return err
	}

	if err := device.Verify(image); err != nil {
		return err
	}

	fmt.Printf("Flashed and verified %d bytes from %s\n", len(image), fs.Arg(0))

	return nil
}

func openPort(name string) (serial.Port, error) {
	port, err := serial.Open(name, &serialMode)
	if err != nil {
		return nil, err
	}

	// Wait for the arduino to finish resetting
	time.Sleep(2 * time.Second)

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
