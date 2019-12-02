package goprotocol

import (
	"net"
	"strconv"
	"time"
)

const timeoutErrorString = "i/o timeout"

type udpConnector struct {
	senderAddress string
	senderPort    int
	receiverPort  int
	udpSender     *net.UDPConn
	udpReceiver   *net.UDPConn
	timeout       time.Duration
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

func (connector *udpConnector) Write(buffer []byte, timestamp time.Time) (StatusCode, int, error) {
	n, err := connector.udpSender.Write(buffer)
	return Success, n, err
}

func (connector *udpConnector) Read(buffer []byte, timestamp time.Time) (StatusCode, int, error) {
	var deadline time.Time
	if connector.timeout > 0 {
		deadline = timestamp.Add(connector.timeout)
	} else {
		deadline = timeZero
	}
	err := connector.udpReceiver.SetReadDeadline(deadline)
	reportError(err)
	n, err := connector.udpReceiver.Read(buffer)
	if err != nil {
		switch err.(type) {
		case *net.OpError:
			if err.(*net.OpError).Err.Error() == timeoutErrorString {
				return Timeout, n, nil
			}
		}
		return Fail, n, err
	}
	return Success, n, err
}

func (connector *udpConnector) SetReadTimeout(t time.Duration) {
	connector.timeout = t
}
