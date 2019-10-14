package protocol

import (
	"log"
	"net"
	"strconv"
)

type connection struct {
	extensionDelegator
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
	buffer := make([]byte, segmentMtu)
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

func Connect(connector Connector) connection {
	connection := connection{}
	arqWriter := &goBackNArqWriter{}
	arqWriter.init()
	arqReader := &goBackNArqReader{}
	adapter := &connectorAdapter{connector}
	connection.addExtension(arqWriter)
	arqWriter.addExtension(arqReader)
	arqReader.addExtension(adapter)
	return connection
}

func UdpConnect(address string, senderPort, receiverPort int) connection {
	var connector Connector = &udpConnector{
		senderAddress: address,
		senderPort:    senderPort,
		receiverPort:  receiverPort,
	}
	return Connect(connector)
}

func handleError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
