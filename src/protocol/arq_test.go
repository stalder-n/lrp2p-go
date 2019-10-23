package protocol

import (
	"container/list"
	"github.com/stretchr/testify/suite"
	"io"
	"testing"
	"time"
)

type GoBackNArqTestSuite struct {
	suite.Suite
	alphaConnection, betaConnection    *connection
	alphaArq, betaArq                  *goBackNArq
	alphaManipulator, betaManipulator2 *segmentManipulator
	initialSequenceNumberQueue         queue
}

func (suite *GoBackNArqTestSuite) SetupTest() {
	suite.mockConnections()
	suite.initialSequenceNumberQueue = queue{}
	sequenceNumberFactory = func() uint32 {
		return suite.initialSequenceNumberQueue.Dequeue().(uint32)
	}
}

func (suite *GoBackNArqTestSuite) TearDownTest() {
	suite.alphaConnection.Close()
	suite.betaConnection.Close()
	setSegmentMtu(defaultSegmentMtu)
}

func (suite *GoBackNArqTestSuite) write(w io.Writer, data []byte) {
	_, err := w.Write(data)
	suite.handleTestError(err)
}

func (suite *GoBackNArqTestSuite) read(r io.Reader, expected string, readBuffer []byte) {
	n, err := r.Read(readBuffer)
	suite.handleTestError(err)
	suite.Equal(expected, string(readBuffer[:n]))
}

func (suite *GoBackNArqTestSuite) readAck(r io.Reader, readBuffer []byte) {
	suite.readExpectError(r, &ackReceivedError{}, readBuffer)
}

func (suite *GoBackNArqTestSuite) readExpectError(r io.Reader, expected error, readBuffer []byte) {
	_, err := r.Read(readBuffer)
	suite.IsType(expected, err)
}

func (suite *GoBackNArqTestSuite) handleTestError(err error) {
	if err != nil {
		suite.T().Error("Error occurred:", err.Error())
	}
}

func (suite *GoBackNArqTestSuite) mockConnections() {
	endpoint1, endpoint2 := make(chan []byte, 100), make(chan []byte, 100)
	connector1, connector2 := &channelConnector{
		in:  endpoint1,
		out: endpoint2,
	}, &channelConnector{
		in:  endpoint2,
		out: endpoint1,
	}
	suite.alphaConnection, suite.alphaArq, suite.alphaManipulator = newMockConnection(connector1)
	suite.betaConnection, suite.betaArq, suite.betaManipulator2 = newMockConnection(connector2)
	suite.alphaConnection.Open()
	suite.betaConnection.Open()
}

func (suite *GoBackNArqTestSuite) TestSendInOneSegment() {
	suite.initialSequenceNumberQueue.Enqueue(uint32(1))
	message := "Hello, World!"
	writeBuffer := []byte(message)
	readBuffer := make([]byte, segmentMtu)
	suite.write(suite.alphaConnection, writeBuffer)
	suite.read(suite.betaConnection, message, readBuffer)
	suite.readAck(suite.alphaConnection, readBuffer)
	suite.Equal(0, suite.alphaArq.lastSegmentAcked)
}

func (suite *GoBackNArqTestSuite) TestRetransmissionByTimeout() {
	suite.initialSequenceNumberQueue.Enqueue(uint32(1))
	suite.alphaManipulator.dropOnce(1)
	retransmissionTimeout = 20 * time.Millisecond
	message := "Hello, World!"
	writeBuffer := []byte(message)
	readBuffer := make([]byte, segmentMtu)
	suite.write(suite.alphaConnection, writeBuffer)
	time.Sleep(retransmissionTimeout)
	suite.write(suite.alphaConnection, nil)
	suite.read(suite.betaConnection, message, readBuffer)
	suite.readAck(suite.alphaConnection, readBuffer)
	suite.Equal(0, suite.alphaArq.lastSegmentAcked)
}

func (suite *GoBackNArqTestSuite) TestSendSegmentsInOrder() {
	suite.initialSequenceNumberQueue.Enqueue(uint32(1))
	setSegmentMtu(headerSize + 4)
	message := "testTESTtEsT"
	writeBuffer := []byte(message)
	readBuffer := make([]byte, segmentMtu)
	suite.write(suite.alphaConnection, writeBuffer)
	suite.read(suite.betaConnection, "test", readBuffer)
	suite.readAck(suite.alphaConnection, readBuffer)
	suite.read(suite.betaConnection, "TEST", readBuffer)
	suite.readAck(suite.alphaConnection, readBuffer)
	suite.read(suite.betaConnection, "tEsT", readBuffer)
	suite.readAck(suite.alphaConnection, readBuffer)
	suite.Equal(2, suite.alphaArq.lastSegmentAcked)
}

func (suite *GoBackNArqTestSuite) TestSendSegmentsOutOfOrder() {
	suite.initialSequenceNumberQueue.Enqueue(uint32(1))
	setSegmentMtu(headerSize + 4)
	suite.alphaManipulator.dropOnce(2)
	message := "testTESTtEsT"
	writeBuffer := []byte(message)
	readBuffer := make([]byte, segmentMtu)
	suite.write(suite.alphaConnection, writeBuffer)
	suite.read(suite.betaConnection, "test", readBuffer)
	suite.readAck(suite.alphaConnection, readBuffer)
	suite.readExpectError(suite.betaConnection, &invalidSegmentError{}, readBuffer)
	suite.readAck(suite.alphaConnection, readBuffer)
	suite.write(suite.alphaConnection, nil)
	suite.read(suite.betaConnection, "TEST", readBuffer)
	suite.readAck(suite.alphaConnection, readBuffer)
	suite.read(suite.betaConnection, "tEsT", readBuffer)
	suite.readAck(suite.alphaConnection, readBuffer)
	suite.Equal(2, suite.alphaArq.lastSegmentAcked)
}

func TestGoBackNArq(t *testing.T) {
	suite.Run(t, new(GoBackNArqTestSuite))
}

func setSegmentMtu(mtu int) {
	segmentMtu = mtu
	dataChunkSize = segmentMtu - headerSize
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
