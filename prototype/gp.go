package main

import (
	"log"
	"net"
	"strconv"
)

type Connection struct {
	sender   *net.UDPConn
	receiver *net.UDPConn
}

func createUdpAddress(addressString string, port int) *net.UDPAddr {
	address := addressString + ":" + strconv.Itoa(port)
	udpAddress, err := net.ResolveUDPAddr("udp4", address)
	handleError(err)

	return udpAddress
}

func Connect(addressString string, senderPort, receiverPort int) *Connection {
	senderAddress := createUdpAddress(addressString, senderPort)
	receiverAddress := createUdpAddress("localhost", receiverPort)
	sender, err := net.DialUDP("udp4", nil, senderAddress)
	handleError(err)
	receiver, err := net.ListenUDP("udp4", receiverAddress)
	handleError(err)

	return &Connection{sender, receiver}
}

func (connection *Connection) Send(str string) {
	seg := createDefaultSegment(0, str)
	_, err := connection.sender.Write(seg.buffer)
	handleError(err)
}

func (connection *Connection) Receive() string {
	buffer := make([]byte, SegmentMtu)
	_, err := connection.receiver.Read(buffer)
	handleError(err)
	seg := createSegment(buffer)
	return seg.getDataAsString()
}

func (connection *Connection) Close() {
	err1 := connection.sender.Close()
	err2 := connection.receiver.Close()
	handleError(err1)
	handleError(err2)
}

func handleError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
