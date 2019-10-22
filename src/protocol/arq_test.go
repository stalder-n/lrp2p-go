package protocol

import (
	"container/list"
	"github.com/stretchr/testify/assert"
	"io"
	"testing"
	"time"
)

var alphaConnection, betaConnection *connection
var alphaArq, betaArq *goBackNArq
var alphaManipulator, betaManipulator2 *segmentManipulator
var initialSequenceNumberQueue queue

func TestGoBackNArq_SendInOneSegment(t *testing.T) {
	setup()
	defer teardown()
	initialSequenceNumberQueue.Enqueue(uint32(1))
	message := "Hello, World!"
	writeBuffer := []byte(message)
	readBuffer := make([]byte, segmentMtu)
	write(t, alphaConnection, writeBuffer)
	read(t, betaConnection, message, readBuffer)
	readAck(t, alphaConnection, readBuffer)
	assert.Equal(t, 0, alphaArq.lastSegmentAcked)
}

func TestGoBackNArq_RetransmissionByTimeout(t *testing.T) {
	setup()
	defer teardown()
	initialSequenceNumberQueue.Enqueue(uint32(1))
	alphaManipulator.dropOnce(1)
	retransmissionTimeout = 20 * time.Millisecond
	message := "Hello, World!"
	writeBuffer := []byte(message)
	readBuffer := make([]byte, segmentMtu)
	write(t, alphaConnection, writeBuffer)
	time.Sleep(retransmissionTimeout)
	write(t, alphaConnection, nil)
	read(t, betaConnection, message, readBuffer)
	readAck(t, alphaConnection, readBuffer)
	assert.Equal(t, 0, alphaArq.lastSegmentAcked)
}

func TestGoBackNArq_SendSegmentsInOrder(t *testing.T) {
	setup()
	defer teardown()
	initialSequenceNumberQueue.Enqueue(uint32(1))
	setSegmentMtu(headerSize + 4)
	message := "testTESTtEsT"
	writeBuffer := []byte(message)
	readBuffer := make([]byte, segmentMtu)
	write(t, alphaConnection, writeBuffer)
	read(t, betaConnection, "test", readBuffer)
	readAck(t, alphaConnection, readBuffer)
	read(t, betaConnection, "TEST", readBuffer)
	readAck(t, alphaConnection, readBuffer)
	read(t, betaConnection, "tEsT", readBuffer)
	readAck(t, alphaConnection, readBuffer)
	assert.Equal(t, 2, alphaArq.lastSegmentAcked)
}

func TestGoBackNArq_SendSegmentsOutOfOrder(t *testing.T) {
	setup()
	defer teardown()
	initialSequenceNumberQueue.Enqueue(uint32(1))
	setSegmentMtu(headerSize + 4)
	alphaManipulator.dropOnce(2)
	message := "testTESTtEsT"
	writeBuffer := []byte(message)
	readBuffer := make([]byte, segmentMtu)
	write(t, alphaConnection, writeBuffer)
	read(t, betaConnection, "test", readBuffer)
	readAck(t, alphaConnection, readBuffer)
	readExpectError(t, betaConnection, &invalidSegmentError{}, readBuffer)
	readAck(t, alphaConnection, readBuffer)
	write(t, alphaConnection, nil)
	read(t, betaConnection, "TEST", readBuffer)
	readAck(t, alphaConnection, readBuffer)
	read(t, betaConnection, "tEsT", readBuffer)
	readAck(t, alphaConnection, readBuffer)
	assert.Equal(t, 2, alphaArq.lastSegmentAcked)
}

func setSegmentMtu(mtu int) {
	segmentMtu = mtu
	dataChunkSize = segmentMtu - headerSize
}

func write(t *testing.T, w io.Writer, data []byte) {
	_, err := w.Write(data)
	handleTestError(err, t)
}

func read(t *testing.T, r io.Reader, expected string, readBuffer []byte) {
	n, err := r.Read(readBuffer)
	handleTestError(err, t)
	assert.Equal(t, expected, string(readBuffer[:n]))
}

func readAck(t *testing.T, r io.Reader, readBuffer []byte) {
	readExpectError(t, r, &ackReceivedError{}, readBuffer)
}

func readExpectError(t *testing.T, r io.Reader, expected error, readBuffer []byte) {
	_, err := r.Read(readBuffer)
	assert.IsType(t, expected, err)
}

func handleTestError(err error, t *testing.T) {
	if err != nil {
		t.Error("Error occurred:", err.Error())
	}
}

func setup() {
	mockConnections()
	initialSequenceNumberQueue = queue{}
	sequenceNumberFactory = func() uint32 {
		return initialSequenceNumberQueue.Dequeue().(uint32)
	}
}

func teardown() {
	alphaConnection.Close()
	betaConnection.Close()
	setSegmentMtu(defaultSegmentMtu)
}

func mockConnections() {
	endpoint1, endpoint2 := make(chan []byte, 100), make(chan []byte, 100)
	connector1, connector2 := &channelConnector{
		in:  endpoint1,
		out: endpoint2,
	}, &channelConnector{
		in:  endpoint2,
		out: endpoint1,
	}
	alphaConnection, alphaArq, alphaManipulator = newMockConnection(connector1)
	betaConnection, betaArq, betaManipulator2 = newMockConnection(connector2)
	alphaConnection.Open()
	betaConnection.Open()
}

func newMockConnection(connector *channelConnector) (*connection, *goBackNArq, *segmentManipulator) {
	connection := &connection{}
	arq := &goBackNArq{}
	manipulator := &segmentManipulator{}
	adapter := &connectorAdapter{connector}
	connection.addExtension(arq)
	arq.addExtension(manipulator)
	manipulator.addExtension(adapter)
	return connection, arq, manipulator
}

type segmentManipulator struct {
	extensionDelegator
	savedSegments map[uint32][]byte
	toDropOnce    list.List
}

func (manipulator *segmentManipulator) dropOnce(sequenceNumber uint32) {
	manipulator.toDropOnce.PushFront(sequenceNumber)
}

func (manipulator *segmentManipulator) Write(buffer []byte) (int, error) {
	seg := createSegment(buffer)
	for elem := manipulator.toDropOnce.Front(); elem != nil; elem = elem.Next() {
		if elem.Value.(uint32) == seg.getSequenceNumber() {
			manipulator.toDropOnce.Remove(elem)
			return len(buffer), nil
		}
	}
	return manipulator.extension.Write(buffer)
}

type channelConnector struct {
	in  chan []byte
	out chan []byte
}

func (connector *channelConnector) Open() {
}

func (connector *channelConnector) Close() {
	close(connector.in)
}

func (connector *channelConnector) Write(buffer []byte) (int, error) {
	connector.out <- buffer
	return len(buffer), nil
}

func (connector *channelConnector) Read(buffer []byte) (int, error) {
	buff := <-connector.in
	copy(buffer, buff)
	return len(buff), nil

}
