package atp

/*
 1. Listen on port xy -> PeerSocket (possibility to close, connect)
 2. PeerSocket.Close -> close locally, send best-effort close message: other side removes mapping for multiplexing,
    notifies user, (multiplexing, GC, keep-alive)
 3. PeerSocket.Connect -> register Connect struct with mapping for multiplexing, store remote port/ip/connid. Handshake
    exchange key material
    (amp attack: I'm 2.2.2.2, attacker is 6.6.6.6, victim is 7.7.7.7
    -> attacker sends UDP packet to 2.2.2.2 with source IP 7.7.7.7 handshake is "hello/ueoeauaoue"
    -> 2.2.2.2 replies "hello there", twice as large to 7.7.7.7
 4. Connect.Write([]byte) error, Connect.Read([]byte) (int, error) -> go interface
     * ARQ.Write -> header, seg
     * Connect.Write -> connectionId, header, seg
     * Security.Write -> nonce + enc(connectionId, header, seg)
 5. Connect.Read([]byte) (int, error)
     * Security.Read
     * Connect.Read -> header seg
     * ARQ.Read -> []byte
*/

import (
	"bytes"
	"log"
	"net"
	"strconv"
	"sync"
	"time"
)

const (
	defaultMTU   = 1400
	headerLength = 6
	authDataSize = 64
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

const (
	timeoutErrorString          = "i/o timeout"
	connectionClosedErrorString = "use of closed network connection"
)

var timeZero = time.Time{}

const retryTimeout = 10 * time.Millisecond

type position struct {
	Start int
	End   int
}

type udpConnector struct {
	server       *net.UDPConn
	remoteAddr   *net.UDPAddr
	timeout      time.Duration
	errorChannel chan error
}

// Socket is an ATP Socket that can open a two-way connection to
// another Socket. Use atp.SocketConnect to create an instance.
type Socket struct {
	readBuffer    bytes.Buffer
	readNotifier  chan bool
	mutex         sync.Mutex
	timeout       time.Duration
	isReadWriting bool
	errorChannel  chan error
	multiplex     map[uint64]*selectiveArq
}

type connector interface {
	Read([]byte, time.Time) (statusCode, int, error)
	Write([]byte, time.Time) (statusCode, int, error)
	Close() error
	SetReadTimeout(time.Duration)
	ConnectTo(remoteHost string, remotePort int)
	reportError(error)
}

type conn struct {
}

// TODO change to reportError(error, chan error) and replace calls with connector.reportError where possible
func reportError(err error) {
	if err != nil {
		log.Println(err)
	}
}

func connect(connector connector, errors chan error) *selectiveArq {
	sec := newSecurityExtension(connector, nil, nil, errors)
	arq := newSelectiveArq(sec, errors)
	return arq
}

func handleError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func udpListen(host string, localPort int, errorChannel chan error) (*udpConnector, error) {
	localAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(host, strconv.Itoa(localPort)))
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

func (connector *udpConnector) ConnectTo(remoteHost string, remotePort int) {
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

// SocketListen creates a socket listening on the specified local port for a connection
func SocketListen(host string, localPort int) *Socket {
	errorChannel := make(chan error, 100)
	connector, err := udpListen(host, localPort, errorChannel)
	reportError(err)
	return newSocket(connector, errorChannel)
}

func newSocket(connector connector, errorChannel chan error) *Socket {
	return &Socket{
		connection:   connect(connector, errorChannel),
		readNotifier: make(chan bool, 1),
		errorChannel: errorChannel,
	}
}

// ConnectTo points this socket to the specified remote host and port
func (socket *Socket) ConnectTo(remoteHost string, remotePort int) {
	socket.connection.ConnectTo(remoteHost, remotePort)
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
		go socket.write()
		go socket.read()
		go socket.checkForSegmentTimeout()
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
		go socket.checkForSegmentTimeout()
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
		if socket.connection.sRtt == 0 {
			select {
			case <-time.After(socket.connection.rto):
				_, _, err := socket.connection.Write(nil, time.Now())
				reportError(err)
			}
		} else {
			select {
			case <-time.After(time.Duration(socket.connection.sRtt)):
				_, _, err := socket.connection.Write(nil, time.Now())
				reportError(err)
			}
		}
	}
}
