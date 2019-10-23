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
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	Open() error
	Close() error
}

type udpConnector struct {
	senderAddress string
	senderPort    int
	receiverPort  int
	udpSender     *net.UDPConn
	udpReceiver   *net.UDPConn
}

func (connector *udpConnector) Open() error {
	senderAddress := createUdpAddress(connector.senderAddress, connector.senderPort)
	receiverAddress := createUdpAddress("localhost", connector.receiverPort)
	var err error = nil
	connector.udpSender, err = net.DialUDP("udp4", nil, senderAddress)
	if err != nil {
		return err
	}
	connector.udpReceiver, err = net.ListenUDP("udp4", receiverAddress)
	return err
}

func (connector *udpConnector) Close() error {
	senderError := connector.udpSender.Close()
	receiverError := connector.udpReceiver.Close()
	if senderError != nil {
		return senderError
	}
	return receiverError
}

func (connector *udpConnector) Write(buffer []byte) (int, error) {
	return connector.udpSender.Write(buffer)
}

func (connector *udpConnector) Read(buffer []byte) (int, error) {
	return connector.udpReceiver.Read(buffer)
}

func createUdpAddress(addressString string, port int) *net.UDPAddr {
	address := addressString + ":" + strconv.Itoa(port)
	udpAddress, err := net.ResolveUDPAddr("udp4", address)
	handleError(err)
	return udpAddress
}

func Connect(connector Connector) *connection {
	connection := &connection{}
	arq := &goBackNArq{}
	adapter := &connectorAdapter{connector}
	connection.addExtension(arq)
	arq.addExtension(adapter)
	return connection
}

func UdpConnect(address string, senderPort, receiverPort int) *connection {
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
