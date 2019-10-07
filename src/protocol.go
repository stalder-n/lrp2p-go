package main

import (
	"log"
	"net"
	"strconv"
)

type Connection struct {
	extension *extension
}

func (connection *Connection) Open() {
	(*connection.extension).Open()
}

func (connection *Connection) Close() {
	(*connection.extension).Close()
}

func (connection *Connection) Write(buffer []byte) {
	(*connection.extension).Write(buffer)
}

func (connection *Connection) Read() []byte {
	return (*connection.extension).Read()
}

func (connection *Connection) AddExtension(extension *extension) {
	connection.extension = extension
}

type Connector interface {
	Open()
	Close()
	Write(buffer []byte)
	Read() []byte
}

type udpConnector struct {
	senderAddress string
	senderPort    int
	receiverPort  int
	udpSender     *net.UDPConn
	udpReceiver   *net.UDPConn
}

func (connector *udpConnector) Open() {
	senderAddress := createUdpAddress(connector.senderAddress, connector.senderPort)
	receiverAddress := createUdpAddress("localhost", connector.receiverPort)
	var err error = nil
	connector.udpSender, err = net.DialUDP("udp4", nil, senderAddress)
	handleError(err)
	connector.udpReceiver, err = net.ListenUDP("udp4", receiverAddress)
	handleError(err)
}

func (connector *udpConnector) Close() {
	senderError := connector.udpSender.Close()
	receiverError := connector.udpReceiver.Close()
	handleError(senderError)
	handleError(receiverError)
}

func (connector *udpConnector) Write(buffer []byte) {
	_, err := connector.udpSender.Write(buffer)
	handleError(err)
}

func (connector *udpConnector) Read() []byte {
	buffer := make([]byte, SegmentMtu)
	n, err := connector.udpReceiver.Read(buffer)
	handleError(err)
	return buffer[:n]
}

func createUdpAddress(addressString string, port int) *net.UDPAddr {
	address := addressString + ":" + strconv.Itoa(port)
	udpAddress, err := net.ResolveUDPAddr("udp4", address)
	handleError(err)

	return udpAddress
}

func Connect(connector *Connector) Connection {
	var adapter extension = &connectorAdapter{connector}
	return Connection{
		extension: &adapter,
	}
}

func UdpConnect(address string, senderPort, receiverPort int) Connection {
	var connector Connector = &udpConnector{
		senderAddress: address,
		senderPort:    senderPort,
		receiverPort:  receiverPort,
	}
	return Connect(&connector)
}

func handleError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
