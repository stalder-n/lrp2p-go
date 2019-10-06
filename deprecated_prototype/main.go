package main

import (
	"fmt"
	"sync"
)

func main() {
	connection1 := Connect("localhost", 3031, 3030)
	connection2 := Connect("localhost", 3030, 3031)
	defer connection1.Close()
	defer connection2.Close()

	var mutex = sync.WaitGroup{}
	mutex.Add(2)
	go func() {
		connection1.Send("Hello")
		fmt.Println("received:", connection1.Receive())
		mutex.Done()
	}()
	go func() {
		connection2.Send("World")
		fmt.Println("received:", connection2.Receive())
		mutex.Done()
	}()

	mutex.Wait()
}
