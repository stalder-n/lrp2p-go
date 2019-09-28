package main

import (
	"log"
	"net"
	"strconv"
)

func createUdpAddress(destination string, port int) *net.UDPAddr {
	address := destination + ":" + strconv.Itoa(port)
	udpAddress, error := net.ResolveUDPAddr("udp4", address)
	if error != nil {
		log.Fatal(error)
	}

	return udpAddress
}

func createUdpListener(udpAddress *net.UDPAddr) *net.UDPConn {
	listener, error := net.ListenUDP("udp", udpAddress)
	if error != nil {
		log.Fatal(error)
	}

	return listener
}

func createUdpSender(udpAddress *net.UDPAddr) *net.UDPConn {
	connection, error := net.DialUDP("udp", nil, udpAddress)
	if error != nil {
		log.Fatal(error)
	}

	return connection
}
