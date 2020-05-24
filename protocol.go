package atp

import (
	"bytes"
	"crypto/rand"
	"log"
	"net"
	"strconv"
	"sync"
	"time"
)

const (
	flagACK byte = 1
	flagSYN byte = 2
)

const (
	defaultMTU   = 1024
	headerLength = 6
)

type statusCode int

const (
	success statusCode = iota
	fail
	ackReceived
	invalidSegment
	windowFull
	waitingForHandshake
	invalidNonce
	timeout
	connectionClosed
)

type position struct {
	Start int
	End   int
}

var retransmissionTimeout = 200 * time.Millisecond
var timeoutCheckInterval = 100 * time.Millisecond

var timeZero = time.Time{}

var generateRandomSequenceNumber = func() uint32 {
	b := make([]byte, 4)
	_, err := rand.Read(b[2:])
	handleError(err)
	sequenceNum := bytesToUint32(b)
	if sequenceNum == 0 {
		sequenceNum++
	}
	return sequenceNum
}

// TODO change to reportError(error, chan error) and replace calls with connector.reportError where possible
func reportError(err error) {
	if err != nil {
		log.Println(err)
	}
}

type connector interface {
	Read([]byte, time.Time) (statusCode, int, error)
	Write([]byte, time.Time) (statusCode, int, error)
	Close() error
	SetReadTimeout(time.Duration)
	Dial(remoteHost string, remotePort int)
	reportError(error)
}

func dial(connector connector, errors chan error) *selectiveArq {
	sec := newSecurityExtension(connector, nil, nil, errors)
	arq := newSelectiveArq(generateRandomSequenceNumber(), sec, errors)
	return arq
}

func handleError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

type udpConnector struct {
	server       *net.UDPConn
	remoteAddr   *net.UDPAddr
	timeout      time.Duration
	errorChannel chan error
}

const timeoutErrorString = "i/o timeout"
const connectionClosedErrorString = "use of closed network connection"

func udpListen(localPort int, errorChannel chan error) (*udpConnector, error) {
	localAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort("", strconv.Itoa(localPort)))
	connection, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		return nil, err
	}

	connector := &udpConnector{
		server:       connection,
		timeout:      0,
		errorChannel: errorChannel,
	}

	return connector, nil
}

func createUDPAddress(addressString string, port int) *net.UDPAddr {
	address := net.JoinHostPort(addressString, strconv.Itoa(port))
	udpAddress, err := net.ResolveUDPAddr("udp4", address)
	handleError(err)
	return udpAddress
}

func (connector *udpConnector) Close() error {
	return connector.server.Close()
}
func (connector *udpConnector) Write(buffer []byte, timestamp time.Time) (statusCode, int, error) {
	if connector.remoteAddr == nil {
		return fail, 0, nil
	}
	n, err := connector.server.WriteToUDP(buffer, connector.remoteAddr)
	if err != nil {
		return fail, n, err
	}
	return success, n, err
}

func (connector *udpConnector) Dial(remoteHost string, remotePort int) {
	connector.remoteAddr = createUDPAddress(remoteHost, remotePort)
}

func (connector *udpConnector) Read(buffer []byte, timestamp time.Time) (statusCode, int, error) {
	var deadline time.Time
	if connector.timeout > 0 {
		deadline = timestamp.Add(connector.timeout)
	} else {
		deadline = timeZero
	}
	err := connector.server.SetReadDeadline(deadline)
	connector.reportError(err)
	n, addr, err := connector.server.ReadFromUDP(buffer)
	if connector.remoteAddr == nil {
		connector.remoteAddr = addr
	}
	if err != nil {
		switch err.(type) {
		case *net.OpError:
			if err.(*net.OpError).Err.Error() == timeoutErrorString {
				return timeout, n, nil
			} else if err.(*net.OpError).Err.Error() == connectionClosedErrorString {
				return connectionClosed, 0, nil
			}
		}
		return fail, n, err
	}
	return success, n, err
}

func (connector *udpConnector) SetReadTimeout(t time.Duration) {
	connector.timeout = t
}

func (connector *udpConnector) reportError(err error) {
	if err != nil {
		connector.errorChannel <- err
	}
}

// Socket is an ATP Socket that can open a two-way connection to
// another Socket. Use atp.SocketConnect to create an instance.
type Socket struct {
	connection    *selectiveArq
	readBuffer    bytes.Buffer
	readNotifier  chan bool
	mutex         sync.Mutex
	timeout       time.Duration
	isReadWriting bool
	errorChannel  chan error
}

const retryTimeout = 10 * time.Millisecond

// SocketListen creates a socket listening on the specified local port for a connection
func SocketListen(localPort int) *Socket {
	errorChannel := make(chan error, 100)
	connector, err := udpListen(localPort, errorChannel)
	reportError(err)
	return newSocket(connector, errorChannel)
}

func newSocket(connector connector, errorChannel chan error) *Socket {
	return &Socket{
		connection:   dial(connector, errorChannel),
		readNotifier: make(chan bool, 1),
		errorChannel: errorChannel,
	}
}

// Dial points this socket to the specified remote host and port
func (socket *Socket) Dial(remoteHost string, remotePort int) {
	socket.connection.Dial(remoteHost, remotePort)
}

// GetNextError returns the next internal error that occurred, if any is available.
// As this reads from the underlying error channel used to propagate errors
// that cannot be properly returned, this method will block while no error is
// available.
func (socket *Socket) GetNextError() error {
	return <-socket.errorChannel
}

// TryGetNextError returns the next internal error that occurred. If no errors
// are found, nil is returned instead instead of blocking
func (socket *Socket) TryGetNextError() error {
	if len(socket.errorChannel) > 0 {
		return <-socket.errorChannel
	}
	return nil
}

// Close closes the underlying two-way connection interface, preventing all
// future calls to Socket.Write and Socket.Read from having any effect
func (socket *Socket) Close() error {
	return socket.connection.Close()
}

// Write writes the specified buffer to the socket's underlying connection
func (socket *Socket) Write(buffer []byte) (int, error) {
	_, _, err := socket.connection.Write(buffer, time.Now())
	if !socket.isReadWriting {
		go socket.read()
		go socket.write()
		socket.isReadWriting = true
	}
	return len(buffer), err
}

func (socket *Socket) write() {
	for {
		statusCode, _, _ := socket.connection.Write(nil, time.Now())
		switch statusCode {
		case connectionClosed:
			return
		}
		time.Sleep(retryTimeout)
	}
}

// Read reads from the underlying connection interface
func (socket *Socket) Read(buffer []byte) (int, error) {
	if !socket.isReadWriting {
		go socket.read()
		go socket.write()
		socket.isReadWriting = true
	}
	socket.mutex.Lock()
	for socket.readBuffer.Len() == 0 {
		socket.mutex.Unlock()
		if socket.timeout > 0 {
			select {
			case <-socket.readNotifier:
				socket.mutex.Lock()
			case <-time.After(socket.timeout):
				return 0, nil
			}
		} else {
			select {
			case <-socket.readNotifier:
				socket.mutex.Lock()
			}
		}
	}
	n, err := socket.readBuffer.Read(buffer)
	socket.mutex.Unlock()
	return n, err
}

func (socket *Socket) read() {
	for {
		buffer := make([]byte, segmentMtu)
		statusCode, n, err := socket.connection.Read(buffer, time.Now())
		socket.connection.reportError(err)
		switch statusCode {
		case success:
			socket.mutex.Lock()
			socket.readBuffer.Write(buffer[:n])
			socket.mutex.Unlock()
			if len(socket.readNotifier) == 0 {
				socket.readNotifier <- true
			}
		case ackReceived:
		case invalidNonce:
		case invalidSegment:
		case connectionClosed:
			return
		}
	}
}

// SetReadTimeout sets an idle timeout for read operations
func (socket *Socket) SetReadTimeout(timeout time.Duration) {
	socket.timeout = timeout
}

func (socket *Socket) checkForSegmentTimeout() {
	for {
		select {
		case <-time.After(timeoutCheckInterval):
			_, _, err := socket.connection.Write(nil, time.Now())
			reportError(err)
		}
	}
}
