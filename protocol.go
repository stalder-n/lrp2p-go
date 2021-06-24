package lrp2p

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
	"container/list"
	"crypto/rand"
	"encoding/binary"
	"io"
	"log"
	"net"
	"strconv"
	"sync"
	"time"
)

type (
	statusCode int
	connState  int
)

const (
	defaultMTU   = 1400
	headerLength = 6
	authDataSize = 64
)

const (
	success statusCode = iota
	fail
	ackReceived
	invalidSegment
	windowFull
	timeout
	connectionClosed
)

const (
	waitingForHandshake connState = iota
	connected
	closed
)

const (
	timeoutErrorString          = "i/o timeout"
	connectionClosedErrorString = "use of closed network connection"
)

const (
	retryTimeout       = 10 * time.Millisecond
	defaultReadTimeout = 10 * time.Millisecond
)

const newConnRejectThresh = 10

var timeZero = time.Time{}

type position struct {
	Start int
	End   int
}

type udpSocket struct {
	local        *net.UDPConn
	timeout      time.Duration
	errorChannel chan error
}

type connector interface {
	io.Closer
	Write([]byte) (statusCode, int, error)
	Read([]byte, time.Time) (statusCode, int, error)
	SetReadTimeout(time.Duration)
}

type udpConnector struct {
	socket     *udpSocket
	remoteAddr *net.UDPAddr
}

type Conn struct {
	endpoint connector
	arq      *selectiveArq
	enc      *encryption
	state    connState
	connId   uint64

	readBuffer   bytes.Buffer
	readNotifier chan bool
	mutex        sync.Mutex
	timeout      time.Duration
}

// PeerSocket is an LRP2P PeerSocket that can open a two-way connection to
// another PeerSocket. Use lrp2p.SocketConnect to create an instance.
type PeerSocket struct {
	udp             *udpSocket
	isReadWriting   bool
	errorChannel    chan error
	multiplex       map[uint64]*Conn
	newConnNotifier *sync.Cond
	newConns        *list.List
}

// TODO change to reportError(error, chan error) and replace calls with connector.reportError where possible
func reportError(err error) {
	if err != nil {
		log.Println(err)
	}
}

func newConnId() uint64 {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return binary.BigEndian.Uint64(b)
}

func newConn(connId uint64, endpoint connector) *Conn {
	endpoint.SetReadTimeout(defaultReadTimeout)
	return &Conn{endpoint: endpoint, readNotifier: make(chan bool, 1), connId: connId}
}

func udpListen(localAddr *net.UDPAddr, errorChannel chan error) (*udpSocket, error) {
	connection, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		return nil, err
	}
	connector := &udpSocket{
		local:        connection,
		timeout:      0,
		errorChannel: errorChannel,
	}

	return connector, nil
}

func createUDPAddress(addressString string, port int) *net.UDPAddr {
	udpAddress, err := net.ResolveUDPAddr("udp4", net.JoinHostPort(addressString, strconv.Itoa(port)))
	reportError(err)
	return udpAddress
}

func (connector *udpSocket) Close() error {
	return connector.local.Close()
}
func (connector *udpSocket) Write(buffer []byte, addr *net.UDPAddr) (statusCode, int, error) {
	n, err := connector.local.WriteToUDP(buffer, addr)
	if err != nil {
		return fail, n, err
	}
	return success, n, err
}

func (connector *udpSocket) Read(buffer []byte, timestamp time.Time) (statusCode, *net.UDPAddr, int, error) {
	var deadline time.Time
	if connector.timeout > 0 {
		deadline = timestamp.Add(connector.timeout)
	} else {
		deadline = timeZero
	}
	err := connector.local.SetReadDeadline(deadline)
	connector.reportError(err)
	n, addr, err := connector.local.ReadFromUDP(buffer)
	if err != nil {
		switch err.(type) {
		case *net.OpError:
			if err.(*net.OpError).Err.Error() == timeoutErrorString {
				return timeout, nil, n, nil
			} else if err.(*net.OpError).Err.Error() == connectionClosedErrorString {
				return connectionClosed, nil, 0, nil
			}
		}
		return fail, nil, n, err
	}
	return success, addr, n, err
}

func (connector *udpSocket) SetReadTimeout(t time.Duration) {
	connector.timeout = t
}

func (connector *udpSocket) reportError(err error) {
	if err != nil {
		connector.errorChannel <- err
	}
}

// SocketListen creates a socket listening on the specified local port for a connection
func SocketListen(host string, localPort int) *PeerSocket {
	errorChannel := make(chan error, 100)
	socket, err := udpListen(createUDPAddress(host, localPort), errorChannel)
	reportError(err)
	peer := &PeerSocket{
		udp:             socket,
		errorChannel:    errorChannel,
		multiplex:       make(map[uint64]*Conn),
		newConnNotifier: sync.NewCond(&sync.Mutex{}),
		newConns:        list.New(),
	}
	go peer.write()
	go peer.read()
	return peer
}

func (socket *PeerSocket) Accept() *Conn {
	socket.newConnNotifier.L.Lock()
	for socket.newConns.Len() == 0 {
		socket.newConnNotifier.Wait()
	}
	conn := socket.newConns.Front().Value.(*Conn)
	socket.newConnNotifier.L.Unlock()

	socket.multiplex[conn.getConnId()] = conn
	state := conn.enc.AcceptHandshake(conn)
	if state != connected {
		delete(socket.multiplex, conn.getConnId())
		return nil
	}
	conn.state = state
	return conn
}

// ConnectTo points this socket to the specified remote host and port
func (socket *PeerSocket) ConnectTo(remoteHost string, remotePort int) *Conn {
	addr := createUDPAddress(remoteHost, remotePort)
	conn := newConn(newConnId(), &udpConnector{socket: socket.udp, remoteAddr: addr})
	conn.arq = newSelectiveArq(conn.writeToUdpEncrypted, socket.errorChannel)
	conn.enc = newEncryption(socket.errorChannel)
	socket.multiplex[conn.getConnId()] = conn
	state := conn.enc.InitiateHandshake(conn)
	if state != connected {
		delete(socket.multiplex, conn.getConnId())
		return nil
	}
	conn.state = state
	return conn
}

// GetNextError returns the next internal error that occurred, if any is available.
// As this reads from the underlying error channel used to propagate errors
// that cannot be properly returned, this method will block while no error is
// available.
func (socket *PeerSocket) GetNextError() error {
	return <-socket.errorChannel
}

// TryGetNextError returns the next internal error that occurred. If no errors
// are found, nil is returned instead instead of blocking
func (socket *PeerSocket) TryGetNextError() error {
	if len(socket.errorChannel) > 0 {
		return <-socket.errorChannel
	}
	return nil
}

// Close closes the underlying two-way connection interface, preventing all
// future calls to PeerSocket.Write and PeerSocket.Read from having any effect
func (socket *PeerSocket) Close() error {
	return socket.udp.Close()
}

func (socket *PeerSocket) write() {
	for {
		for _, c := range socket.multiplex {
			c.arq.retransmitTimedOutSegments(time.Now())
			_, _ = c.arq.writeQueuedSegments(time.Now())
		}
		time.Sleep(retryTimeout)
	}
}

func (socket *PeerSocket) read() {
	for {
		buffer := make([]byte, segmentMtu)
		status, addr, n, err := socket.udp.Read(buffer, time.Now())
		reportError(err)
		if status != success {
			continue
		}
		connId := binary.BigEndian.Uint64(buffer[:8])
		if c, ok := socket.multiplex[connId]; ok {
			if c.state == waitingForHandshake {
				c.enc.messageNotifier.L.Lock()
				c.enc.handshakeMessage = buffer[8:n]
				c.enc.messageNotifier.L.Unlock()
				c.enc.messageNotifier.Signal()
				continue
			}

			n, err = c.enc.Decrypt(buffer[8:n], buffer)
			status, segs := c.arq.processReceivedSegment(buffer[:n], true, time.Now())

			if status == success {
				c.mutex.Lock()
				for _, seg := range segs {
					c.readBuffer.Write(seg.data)
				}
				c.mutex.Unlock()
				if len(c.readNotifier) == 0 {
					c.readNotifier <- true
				}
			}
		} else {
			if socket.newConns.Len() >= newConnRejectThresh {
				continue // reject all incoming connection requests if 10 or more are pending
			}
			conn := newConn(connId, &udpConnector{socket: socket.udp, remoteAddr: addr})
			conn.arq = newSelectiveArq(conn.writeToUdpEncrypted, socket.errorChannel)
			conn.enc = newEncryption(socket.errorChannel)
			conn.enc.handshakeMessage = buffer[8:n]
			socket.newConnNotifier.L.Lock()
			socket.newConns.PushBack(conn)
			socket.newConnNotifier.L.Unlock()
			socket.newConnNotifier.Signal()
		}
	}
}

func (c *Conn) Read(buffer []byte) (int, error) {
	c.mutex.Lock()
	for c.readBuffer.Len() == 0 {
		c.mutex.Unlock()
		if c.timeout > 0 {
			select {
			case <-c.readNotifier:
				c.mutex.Lock()
			case <-time.After(c.timeout):
				return 0, nil
			}
		} else {
			select {
			case <-c.readNotifier:
				c.mutex.Lock()
			}
		}
	}
	n, err := c.readBuffer.Read(buffer)
	c.mutex.Unlock()
	return n, err
}

func (c *Conn) Write(buffer []byte) (int, error) {
	c.arq.queueNewSegments(buffer)
	return len(buffer), nil
}

// SetReadTimeout sets an idle timeout for read operations
func (c *Conn) SetReadTimeout(timeout time.Duration) {
	c.timeout = timeout
}

func (c *Conn) readFromEndpoint(buffer []byte, timestamp time.Time) (statusCode, int, error) {
	return c.endpoint.Read(buffer, timestamp)
}

func (c *Conn) writeToEndpoint(buffer []byte) (statusCode, int, error) {
	return c.endpoint.Write(buffer)
}

func (c *Conn) writeToUdpEncrypted(payload []byte) (statusCode, int, error) {
	buf := make([]byte, segmentMtu)
	n, err := c.enc.Encrypt(payload, buf[8:])
	if err != nil {
		return fail, 0, err
	}
	binary.BigEndian.PutUint64(buf, c.getConnId())
	return c.endpoint.Write(buf[:n+8])
}

func (c *Conn) getConnId() uint64 {
	return c.connId
}

func (udp *udpConnector) Write(buffer []byte) (statusCode, int, error) {
	return udp.socket.Write(buffer, udp.remoteAddr)
}

func (udp *udpConnector) Read(buffer []byte, timestamp time.Time) (statusCode, int, error) {
	status, _, n, err := udp.socket.Read(buffer, timestamp)
	return status, n, err
}

func (udp *udpConnector) Close() error {
	return udp.socket.Close()
}
func (udp *udpConnector) SetReadTimeout(t time.Duration) {
	udp.socket.SetReadTimeout(t)
}
