package protocol

import (
	"sync"
	"time"
)

type Socket struct {
	connection    Connector
	readQueue     concurrencyQueue
	dataAvailable *sync.Cond
}

type payload struct {
	n    int
	err  error
	data []byte
}

func (socket *Socket) Open() error {
	err := socket.connection.Open()
	socket.dataAvailable = sync.NewCond(&sync.Mutex{})
	if err != nil {
		return err
	}
	go socket.read()
	return err
}

func (socket *Socket) Close() error {
	return socket.connection.Close()
}

func (socket *Socket) Write(buffer []byte) (int, error) {
	retryTimeout := 10 * time.Millisecond
	statusCode, n, err := socket.connection.Write(buffer)
	sumN := n

	for statusCode != success {
		if err != nil {
			return sumN, err
		}
		switch statusCode {
		case windowFull:
			time.Sleep(retryTimeout)
			statusCode, n, err = socket.connection.Write(nil)
			sumN += n
		case pendingSegments:
			time.Sleep(retryTimeout)
			statusCode, n, err = socket.connection.Write(buffer)
			sumN += n
		}
	}

	return sumN, err
}

func (socket *Socket) Read(buffer []byte) (int, error) {
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
		buffer := make([]byte, segmentMtu)
		statusCode, n, err := socket.connection.Read(buffer)
		switch statusCode {
		case success:
			p := &payload{n, err, buffer}
			socket.readQueue.Enqueue(p)
			socket.dataAvailable.Signal()
		case ackReceived:
		}
	}
}

func Connect(address string, senderPort, receiverPort int) *Socket {
	var connector Connector = &udpConnector{
		senderAddress: address,
		senderPort:    senderPort,
		receiverPort:  receiverPort,
	}

	return &Socket{connection: connect(connector)}
}
