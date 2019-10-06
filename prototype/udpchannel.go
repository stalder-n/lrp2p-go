package main

import (
	"log"
	"net"
)

type UdpChannel struct {
	Listener *net.UDPConn
	Sender   *net.UDPConn
}

func (channel *UdpChannel) SendMessage(message string) {
	bytes := []byte(message)
	_, error := channel.Sender.Write(bytes)

	if error != nil {
		log.Fatal(error)
	}
}

func (channel *UdpChannel) ReadMessage() []byte {
	buffer := make([]byte, 1024)
	n, error := channel.Listener.Read(buffer)
	buff := buffer[:n]
	if error != nil {
		log.Fatal("n: ", n, buff, error)
	}

	return buff
}

func (channel *UdpChannel) ReadStringMessage() string {
	return string(channel.ReadMessage())
}

func (channel *UdpChannel) Close() {
	channel.Sender.Close()
	channel.Listener.Close()
}

func (channel *UdpChannel) AddExtension(extension *Extension) {

}

func CreateChannel(listenerIp string, listenerPort int, senderIp string, senderPort int) *UdpChannel {
	sourceAddress := createUdpAddress(listenerIp, listenerPort)
	destinationAddress := createUdpAddress(senderIp, senderPort)

	channel := UdpChannel{}
	channel.Listener = createUdpListener(sourceAddress)
	channel.Sender = createUdpSender(destinationAddress)

	return &channel
}
