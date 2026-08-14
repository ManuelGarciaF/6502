package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"go.bug.st/serial"
)

func main() {

	portName := flag.String("p", "/dev/ttyUSB0", "Serial port to connect to the arduino")
	flag.Parse()

	port, err := serial.Open(*portName, &serial.Mode{BaudRate: 250000})
	if err != nil {
		fmt.Println("Unable to open port", *portName + ":", err)
		os.Exit(1)
	}
	defer port.Close()

	fmt.Println("Connected to port:", *portName)

	fmt.Print("Byte to send: 0x")

	s := bufio.NewScanner(os.Stdin)
	for s.Scan() {
		text := strings.TrimSpace(s.Text())
		if text == "" {
			continue
		}
		num, err := strconv.ParseUint(text, 16, 8)
		if err != nil {
			fmt.Println("Invalid input:", text)
			continue
		}

		n, err := port.Write([]byte{byte(num)})
		if err != nil || n != 1 {
			fmt.Println("Error writing to port:", err)
			os.Exit(1)
		}

		fmt.Printf("Sent 0x%02X\n", num)
	    fmt.Print("Byte to send: 0x")
	}
	if err := s.Err(); err != nil {
		fmt.Println("Error reading input:", err)
		os.Exit(1)
	}

}
