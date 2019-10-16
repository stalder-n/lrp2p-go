package main

import (
	"fmt"
	. "protocol"
	"sync"
)

func main() {
	connection1 := UdpConnect("localhost", 3030, 3031)
	connection2 := UdpConnect("localhost", 3031, 3030)
	defer connection1.Close()
	defer connection2.Close()
	connection1.Open()
	connection2.Open()

	var mutex = sync.WaitGroup{}
	mutex.Add(2)
	go func() {
		connection1.Write([]byte("Hello there world, how's it going?"))
		mutex.Done()
	}()
	go func() {
		for {
			fmt.Println("received:", string(connection2.Read()))
		}
		//mutex.Done()
	}()

	mutex.Wait()
}
