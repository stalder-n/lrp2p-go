package goprotocol

import (
	"sync"
	"time"
)

type Socket struct {
	connection    Connector
	readQueue     concurrencyQueue
	dataAvailable *sync.Cond
	isReading     bool
}

func NewSocket(address string, senderPort, receiverPort int) *Socket {
	connector := &udpConnector{
		senderAddress: address,
		senderPort:    senderPort,
		receiverPort:  receiverPort,
	}
	return newSocket(connector, address, senderPort, receiverPort)
}

func newSocket(connector Connector, address string, senderPort, receiverPort int) *Socket {
	socket := &Socket{connection: connect(connector)}
	socket.dataAvailable = sync.NewCond(&sync.Mutex{})
	return socket
}

type payload struct {
	n    int
	err  error
	data []byte
}

func (socket *Socket) Open() error {
	err := socket.connection.Open()
	if err != nil {
		return err
	}

	return err
}

func (socket *Socket) Close() error {
	return socket.connection.Close()
}

func (socket *Socket) Write(buffer []byte) (int, error) {
	retryTimeout := 10 * time.Millisecond
	statusCode, n, err := socket.connection.Write(buffer, time.Now())
	sumN := n

	for statusCode != Success {
		if err != nil {
			return sumN, err
		}
		switch statusCode {
		case WindowFull:
			time.Sleep(retryTimeout)
			statusCode, n, err = socket.connection.Write(nil, time.Now())
			sumN += n
		case PendingSegments:
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
	copy(buffer, p.data)
	return p.n, p.err
}

func (socket *Socket) read() {
	for {
		buffer := make([]byte, SegmentMtu)
		statusCode, n, err := socket.connection.Read(buffer, time.Now())
		switch statusCode {
		case Success:
			p := &payload{n, err, buffer}
			socket.readQueue.Enqueue(p)
			socket.dataAvailable.Signal()
		case AckReceived:
		case InvalidNonce:
		}
	}
}
