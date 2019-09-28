package main

import (
	"log"
	"net"
)

type UdpChannel struct {
	Listener *net.UDPConn
	Sender   *net.UDPConn
	Ordering *OrderingExtension
}

func (channel *UdpChannel) SendMessage(message string) {
	bytes := []byte(message)
	_, error := channel.Sender.Write(bytes)
	if error != nil {
		log.Fatal(error)
	}
}

func (channel *UdpChannel) ReadMessage() []byte {

	if channel.Ordering != nil {
		message, err := channel.Ordering.PopMessage()
		if err == "ok" {
			return message
		}
	}

	buffer := make([]byte, 1024)
	_, error := channel.Listener.Read(buffer)
	if error != nil {
		log.Fatal(error)
	}

	if channel.Ordering != nil {
		channel.Ordering.AddMessage(string(buffer))
	}

	return channel.ReadMessage()
}

func (channel *UdpChannel) ReadStringMessage() string {
	return string(channel.ReadMessage())
}

func CreateChannel(listenerIp string, listenerPort int, senderIp string, senderPort int) *UdpChannel {
	sourceAddress := createUdpAddress(listenerIp, listenerPort)
	destinationAddress := createUdpAddress(senderIp, senderPort)

	channel := UdpChannel{}
	channel.Listener = createUdpListener(sourceAddress)
	channel.Sender = createUdpSender(destinationAddress)

	return &channel
}

func (channel *UdpChannel) AddOrderingExtension(extension *OrderingExtension) {
	channel.Ordering = extension
}

func (channel *UdpChannel) Close() {
	channel.Sender.Close()
	channel.Listener.Close()
}