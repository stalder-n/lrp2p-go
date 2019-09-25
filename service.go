package main

import (
	"log"
	"net"
	"strconv"
)

type Channel struct {
	Listener *net.UDPConn
	Sender   *net.UDPConn
}

/* USAGE:
	c := Channel{};
    c.createChannel(...);
    c.sendMessage(...);
    buffer := c.readMessage();
    c.close();
*/

func (channel *Channel) readMessage() []byte {
	buffer := make([]byte, 1024);
	_, error := channel.Listener.Read(buffer);
	if error != nil {
		log.Fatal(error);
	}
	return buffer;
}

func (channel *Channel) sendMessage(message string) {
	bytes := []byte(message);
	_, error := channel.Sender.Write(bytes);
	if error != nil {
		log.Fatal(error);
	}
}

func (channel *Channel) createChannel(listenerIp string, listenerPort int, senderIp string, senderPort int) {
	sourceAddress := createUdpAddress(listenerIp, listenerPort);
	destinationAddress := createUdpAddress(senderIp, senderPort);
	channel.Listener = createUdpListener(sourceAddress);
	channel.Sender = createUdpSender(destinationAddress);
}

func (channel *Channel) close() {
	channel.Sender.Close();
	channel.Listener.Close();
}

func createUdpAddress(destination string, port int) *net.UDPAddr {
	address := destination + ":" + strconv.Itoa(port);
	udpAddress, error := net.ResolveUDPAddr("udp4", address);
	if error != nil {
		log.Fatal(error)
	}

	return udpAddress;
}

func createUdpListener(udpAddress *net.UDPAddr) *net.UDPConn {
	listener, error := net.ListenUDP("udp", udpAddress);
	if error != nil {
		log.Fatal(error)
	}

	return listener;
}

func createUdpSender(udpAddress *net.UDPAddr) *net.UDPConn {
	connection, error := net.DialUDP("udp", nil, udpAddress);
	if error != nil {
		log.Fatal(error);
	}

	return connection;
}
