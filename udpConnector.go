package atp

import (
	"net"
	"strconv"
	"time"
)

type udpConnector struct {
	udpSender   *net.UDPConn
	udpReceiver *net.UDPConn
	timeout     time.Duration
}

const timeoutErrorString = "i/o timeout"

func newUdpConnector(remoteHostname string, remotePort, localPort int) (*udpConnector, error) {
	remoteAddress := createUdpAddress(remoteHostname, remotePort)
	localAddress := createUdpAddress("localhost", localPort)

	udpSender, err := net.DialUDP("udp4", nil, remoteAddress)
	if err != nil {
		return nil, err
	}
	udpReceiver, err := net.ListenUDP("udp4", localAddress)
	if err != nil {
		return nil, err
	}

	connector := &udpConnector{
		udpSender:   udpSender,
		udpReceiver: udpReceiver,
		timeout:     0,
	}

	return connector, nil
}

func createUdpAddress(addressString string, port int) *net.UDPAddr {
	address := addressString + ":" + strconv.Itoa(port)
	udpAddress, err := net.ResolveUDPAddr("udp4", address)
	handleError(err)
	return udpAddress
}

func (connector *udpConnector) Open() error {
	return nil
}

func (connector *udpConnector) Close() error {
	senderError := connector.udpSender.Close()
	receiverError := connector.udpReceiver.Close()
	if senderError != nil {
		return senderError
	}
	return receiverError
}

func (connector *udpConnector) Write(buffer []byte, timestamp time.Time) (statusCode, int, error) {
	n, err := connector.udpSender.Write(buffer)
	return success, n, err
}

func (connector *udpConnector) Read(buffer []byte, timestamp time.Time) (statusCode, int, error) {
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
				return timeout, n, nil
			}
		}
		return fail, n, err
	}
	return success, n, err
}

func (connector *udpConnector) SetReadTimeout(t time.Duration) {
	connector.timeout = t
}
