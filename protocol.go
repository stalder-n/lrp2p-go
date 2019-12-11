package atp

import (
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
	udpSender    *net.UDPConn
	udpReceiver  *net.UDPConn
	timeout      time.Duration
	errorChannel chan error
}

const timeoutErrorString = "i/o timeout"

func newUDPConnector(remoteHostname string, remotePort, localPort int, errorChannel chan error) (*udpConnector, error) {
	remoteAddress := createUDPAddress(remoteHostname, remotePort)
	localAddress := createUDPAddress("localhost", localPort)

	udpSender, err := net.DialUDP("udp4", nil, remoteAddress)
	if err != nil {
		return nil, err
	}
	udpReceiver, err := net.ListenUDP("udp4", localAddress)
	if err != nil {
		return nil, err
	}

	connector := &udpConnector{
		udpSender:    udpSender,
		udpReceiver:  udpReceiver,
		timeout:      0,
		errorChannel: errorChannel,
	}

	return connector, nil
}

func createUDPAddress(addressString string, port int) *net.UDPAddr {
	address := addressString + ":" + strconv.Itoa(port)
	udpAddress, err := net.ResolveUDPAddr("udp4", address)
	handleError(err)
	return udpAddress
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
	if err != nil {
		return fail, n, err
	}
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

func (connector *udpConnector) reportError(err error) {
	if err != nil {
		connector.errorChannel <- err
	}
}

type payload struct {
	n    int
	err  error
	data []byte
}

type Socket struct {
	connection    connector
	readQueue     concurrencyQueue
	dataAvailable *sync.Cond
	isReading     bool
	errorChannel  chan error
}

func NewSocket(remoteHost string, remotePort, localPort int) *Socket {
	errorChannel := make(chan error, 100)
	connector, err := newUDPConnector(remoteHost, remotePort, localPort, errorChannel)
	reportError(err)
	return newSocket(connector, errorChannel)
}

func newSocket(connector connector, errorChannel chan error) *Socket {
	socket := &Socket{
		connection:    connect(connector, errorChannel),
		readQueue:     *newConcurrencyQueue(),
		dataAvailable: sync.NewCond(&sync.Mutex{}),
		errorChannel:  errorChannel,
	}

	return socket
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

func (socket *Socket) Read(buffer []byte) (int, error) {
	if !socket.isReading {
		go socket.read()
		socket.isReading = true
	}
	socket.dataAvailable.L.Lock()
	for socket.readQueue.IsEmpty() {
		socket.dataAvailable.Wait()
	}
	p := socket.readQueue.Dequeue().(*payload)
	socket.dataAvailable.L.Unlock()
	copy(buffer, p.data[:p.n])
	return p.n, p.err
}

func (socket *Socket) read() {
	for {
		buffer := make([]byte, segmentMtu)
		statusCode, n, err := socket.connection.Read(buffer, time.Now())
		switch statusCode {
		case success:
			p := &payload{n, err, buffer}
			socket.readQueue.Enqueue(p)
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
