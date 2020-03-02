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
	pendingSegments
	invalidSegment
	windowFull
	waitingForHandshake
	invalidNonce
	timeout
)

type position struct {
	Start int
	End   int
}

var dataOffsetPosition = position{0, 1}
var flagPosition = position{1, 2}
var sequenceNumberPosition = position{2, 6}

var retransmissionTimeout = 200 * time.Millisecond
var timeoutCheckInterval = 100 * time.Millisecond

var timeZero = time.Time{}

var generateRandomSequenceNumber = func() uint32 {
	b := make([]byte, 4)
	_, err := rand.Read(b)
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
	ConnectTo(remoteHost string, remotePort int)
	reportError(error)
}

func connect(connector connector, errors chan error) connector {
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
	udpConnection net.PacketConn
	remoteAddr    *net.UDPAddr
	timeout       time.Duration
	errorChannel  chan error
}

const timeoutErrorString = "i/o timeout"

func udpListen(localPort int, errorChannel chan error) (*udpConnector, error) {
	connection, err := net.ListenPacket("udp", net.JoinHostPort("", strconv.Itoa(localPort)))
	if err != nil {
		return nil, err
	}

	connector := &udpConnector{
		udpConnection: connection,
		timeout:       0,
		errorChannel:  errorChannel,
	}

	return connector, nil
}

func udpConnect(remoteHostname string, remotePort, localPort int, errorChannel chan error) (*udpConnector, error) {
	connector, err := udpListen(localPort, errorChannel)
	if err != nil {
		return nil, err
	}
	connector.remoteAddr = createUDPAddress(remoteHostname, remotePort)
	return connector, nil
}

func createUDPAddress(addressString string, port int) *net.UDPAddr {
	address := net.JoinHostPort(addressString, strconv.Itoa(port))
	udpAddress, err := net.ResolveUDPAddr("udp4", address)
	handleError(err)
	return udpAddress
}

func (connector *udpConnector) Close() error {
	return connector.udpConnection.Close()
}

func (connector *udpConnector) Write(buffer []byte, timestamp time.Time) (statusCode, int, error) {
	n, err := connector.udpConnection.WriteTo(buffer, connector.remoteAddr)
	if err != nil {
		return fail, n, err
	}
	return success, n, err
}

func (connector *udpConnector) connectToClient(raddr net.Addr) {
	remote, err := net.ResolveUDPAddr(raddr.Network(), raddr.String())
	connector.reportError(err)
	connector.remoteAddr = remote
}

func (connector *udpConnector) ConnectTo(remoteHost string, remotePort int) {
	remoteAddr := createUDPAddress(remoteHost, remotePort)
	connector.remoteAddr = remoteAddr
}

func (connector *udpConnector) Read(buffer []byte, timestamp time.Time) (statusCode, int, error) {
	var deadline time.Time
	if connector.timeout > 0 {
		deadline = timestamp.Add(connector.timeout)
	} else {
		deadline = timeZero
	}
	err := connector.udpConnection.SetReadDeadline(deadline)
	reportError(err)
	n, raddr, err := connector.udpConnection.ReadFrom(buffer)
	if connector.remoteAddr == nil {
		connector.connectToClient(raddr)
	}
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

func (connector *udpConnector) reportError(err error) {
	if err != nil {
		connector.errorChannel <- err
	}
}

// Socket is an ATP Socket that can open a two-way connection to
// another Socket. Use atp.SocketConnect to create an instance.
type Socket struct {
	connection    connector
	readBuffer    bytes.Buffer
	dataAvailable *sync.Cond
	isReading     bool
	errorChannel  chan error
}

// SocketListen creates a socket listening on the specified local port for a connection
func SocketListen(localPort int) *Socket {
	errorChannel := make(chan error, 100)
	connector, err := udpListen(localPort, errorChannel)
	reportError(err)
	return newSocket(connector, errorChannel)
}

// SocketConnect creates a new ATP Socket instance and sets up a connection
// to the specified host.
func SocketConnect(remoteHost string, remotePort, localPort int) *Socket {
	errorChannel := make(chan error, 100)
	connector, err := udpConnect(remoteHost, remotePort, localPort, errorChannel)
	reportError(err)
	return newSocket(connector, errorChannel)
}

func newSocket(connector connector, errorChannel chan error) *Socket {
	return &Socket{
		connection:    connect(connector, errorChannel),
		dataAvailable: sync.NewCond(&sync.Mutex{}),
		errorChannel:  errorChannel,
	}
}

// ConnectTo points this socket to the specified remote host and port
func (socket *Socket) ConnectTo(remoteHost string, remotePort int) {
	socket.connection.ConnectTo(remoteHost, remotePort)
}

// GetNextError returns the next internal error that occurred, if any is available.
// As this read from the underlying error channel used to propagate errors
// that cannot be properly returned, this method will block while no error
// available.
func (socket *Socket) GetNextError() error {
	return <-socket.errorChannel
}

// Close closes the underlying two-way connection interface, preventing all
// future calls to Socket.Write and Socket.Read from having any effect
func (socket *Socket) Close() error {
	return socket.connection.Close()
}

// Write writes the specified buffer to the socket's underlying connection
func (socket *Socket) Write(buffer []byte) (int, error) {
	retryTimeout := 10 * time.Millisecond
	statusCode, n, err := socket.connection.Write(buffer, time.Now())
	sumN := n
	if !socket.isReading {
		go socket.read()
		socket.isReading = true
	}
	for statusCode != success {
		if err != nil {
			return sumN, err
		}
		switch statusCode {
		case windowFull:
			time.Sleep(retryTimeout)
			statusCode, n, err = socket.connection.Write(nil, time.Now())
			sumN += n
		case pendingSegments:
			time.Sleep(retryTimeout)
			statusCode, n, err = socket.connection.Write(buffer, time.Now())
			sumN += n
		}
	}

	return sumN, err
}

// Read reads from the underlying connection interface
func (socket *Socket) Read(buffer []byte) (int, error) {
	if !socket.isReading {
		go socket.read()
		socket.isReading = true
	}
	socket.dataAvailable.L.Lock()
	for socket.readBuffer.Len() == 0 {
		socket.dataAvailable.Wait()
	}
	n, err := socket.readBuffer.Read(buffer)
	socket.dataAvailable.L.Unlock()
	return n, err
}

// SetReadTimeout sets an idle timeout for read all operations
func (socket *Socket) SetReadTimeout(timeout time.Duration) {
	socket.connection.SetReadTimeout(timeout)
}

func (socket *Socket) read() {
	for {
		buffer := make([]byte, segmentMtu)
		statusCode, n, err := socket.connection.Read(buffer, time.Now())
		socket.connection.reportError(err)
		switch statusCode {
		case success:
			socket.dataAvailable.L.Lock()
			socket.readBuffer.Write(buffer[:n])
			socket.dataAvailable.L.Unlock()
			socket.dataAvailable.Signal()
		case ackReceived:
		case invalidNonce:
		case invalidSegment:
		}
	}
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
