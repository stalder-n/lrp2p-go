package depracated

import (
	"sync"
)

func main() {
	var channel1 Extension = CreateChannel("localhost", 3030, "localhost", 3031)
	var channel2 Extension = CreateChannel("localhost", 3031, "localhost", 3030)

	var orderingExt1 Extension = &OrderingExtension{}
	var orderingExt2 Extension = &OrderingExtension{}

	var reliableEx1 Extension = &ReliableExtension{}
	var reliableEx2 Extension = &ReliableExtension{}

	reliableEx1.AddExtension(&channel1)
	reliableEx2.AddExtension(&channel2)
	orderingExt1.AddExtension(&reliableEx1)
	orderingExt2.AddExtension(&reliableEx2)

	var mutex = sync.WaitGroup{}
	mutex.Add(2)

	go func() {
		orderingExt1.SendMessage("1: I am Pc 1")
		orderingExt1.SendMessage("3: I am Pc 1")
		orderingExt1.SendMessage("2: I am Pc 1")
		orderingExt1.SendMessage("4: fin")

		orderingExt2.SendMessage("1: I am Pc 2")
		orderingExt2.SendMessage("3: I am Pc 2")
		orderingExt2.SendMessage("2: I am Pc 2")
		orderingExt2.SendMessage("4: fin")
	}()
	go func() {
		msg := orderingExt2.ReadStringMessage()
		for {
			println(msg)
			msg = orderingExt2.ReadStringMessage()
		}
		mutex.Done()
	}()
	go func() {
		msg := orderingExt1.ReadStringMessage()
		for {
			println(msg)
			msg = orderingExt1.ReadStringMessage()
		}
		mutex.Done()
	}()
	mutex.Wait()
	reliableEx1.Close()
	reliableEx2.Close()
}
