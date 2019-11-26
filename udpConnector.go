package goprotocol

import (
	"net"
	"strconv"
	"time"
)

type udpConnector struct {
	senderAddress string
	senderPort    int
	receiverPort  int
	udpSender     *net.UDPConn
	udpReceiver   *net.UDPConn
}

func createUdpAddress(addressString string, port int) *net.UDPAddr {
	address := addressString + ":" + strconv.Itoa(port)
	udpAddress, err := net.ResolveUDPAddr("udp4", address)
	handleError(err)
	return udpAddress
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

func (connector *udpConnector) Write(buffer []byte) (StatusCode, int, error) {
	n, err := connector.udpSender.Write(buffer)
	return Success, n, err
}

func (connector *udpConnector) Read(buffer []byte) (StatusCode, int, error) {
	n, err := connector.udpReceiver.Read(buffer)
	return Success, n, err
}

func (connector *udpConnector) SetDeadline(t time.Time) {
	err := connector.udpReceiver.SetReadDeadline(t)
	reportError(err)
}
