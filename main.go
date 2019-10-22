package main

import (
	"fmt"
	. "protocol"
)

func main() {
	connection1 := UdpConnect("localhost", 3030, 3031)
	connection2 := UdpConnect("localhost", 3031, 3030)
	defer connection1.Close()
	defer connection2.Close()
	connection1.Open()
	connection2.Open()

	go func() {
		connection1.Write([]byte("Hello there world, how's it going?"))
		for {
			buf := make([]byte, 64)
			connection1.Read(buf)
			connection1.Write(nil)
		}
	}()
	for {
		buf := make([]byte, 64)
		n, _ := connection2.Read(buf)
		fmt.Println("received:", string(buf[:n]))
	}
}
