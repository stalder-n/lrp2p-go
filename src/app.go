package main

import (
	"sync"
)

func main() {
	channel1 := CreateChannel("localhost", 3030, "localhost", 3031)
	channel2 := CreateChannel("localhost", 3031, "localhost", 3030)

	var channelPc1 Extension = &OrderingExtension{}
	var channelPc2 Extension = &OrderingExtension{}

	channelPc1.AddExtension(channel1)
	channelPc2.AddExtension(channel2)

	var mutex = sync.WaitGroup{}
	mutex.Add(1)

	go func() {
		channelPc1.SendMessage("1: I am Pc 1")
		channelPc1.SendMessage("3: I am Pc 1")
		channelPc1.SendMessage("2: I am Pc 1")

		channelPc2.SendMessage("1: I am Pc 2")
		channelPc2.SendMessage("3: I am Pc 2")
		channelPc2.SendMessage("2: I am Pc 2")
	}()
	go func() {
		println(channelPc1.ReadStringMessage())
		println(channelPc1.ReadStringMessage())
		println(channelPc1.ReadStringMessage())

		println(channelPc2.ReadStringMessage())
		println(channelPc2.ReadStringMessage())
		println(channelPc2.ReadStringMessage())

		mutex.Done()
	}()
	mutex.Wait()
	channelPc1.Close()
	channelPc2.Close()
}
