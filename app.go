package main

import (
	"sync"
)

func main() {
	channelPc1 := Channel{};
	channelPc1.createChannel("localhost", 3030, "localhost", 3031);
	channelPc2 := Channel{};
	channelPc2.createChannel("localhost", 3031, "localhost", 3030);
	var mutex = sync.WaitGroup{}
	mutex.Add(1);

	go func() {
		channelPc1.sendMessage("I am Pc 1");
		channelPc2.sendMessage("I am Pc 2");
	}()
	go func() {
		result1 := channelPc1.readMessage();
		println(string(result1));
		result2 := channelPc2.readMessage();
		println(string(result2));
		mutex.Done();
	}()
	mutex.Wait();
	channelPc1.close();
	channelPc2.close();
}
