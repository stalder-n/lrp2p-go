package protocol

import (
	"container/list"
	"github.com/stretchr/testify/suite"
	"testing"
	"time"
)

type extensionDelegator struct {
	extension Connector
}

func (connection *extensionDelegator) Open() error {
	return connection.extension.Open()
}

func (connection *extensionDelegator) Close() error {
	return connection.extension.Close()
}

func (connection *extensionDelegator) Write(buffer []byte) (statusCode, int, error) {
	return connection.extension.Write(buffer)
}

func (connection *extensionDelegator) Read(buffer []byte) (statusCode, int, error) {
	return connection.extension.Read(buffer)
}

func (connection *extensionDelegator) addExtension(extension Connector) {
	connection.extension = extension
}

type GoBackNArqTestSuite struct {
	suite.Suite
	alphaConnection, betaConnection    *extensionDelegator
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
	suite.handleTestError(suite.alphaConnection.Close())
	suite.handleTestError(suite.betaConnection.Close())
	setSegmentMtu(defaultSegmentMtu)
}

func (suite *GoBackNArqTestSuite) write(c *extensionDelegator, data []byte) {
	status, _, err := c.Write(data)
	suite.handleTestError(err)
	suite.Equal(success, status)
}

func (suite *GoBackNArqTestSuite) read(c *extensionDelegator, expected string, readBuffer []byte) {
	status, n, err := c.Read(readBuffer)
	suite.handleTestError(err)
	suite.Equal(expected, string(readBuffer[:n]))
	suite.Equal(success, status)
}

func (suite *GoBackNArqTestSuite) readAck(c *extensionDelegator, readBuffer []byte) {
	suite.readExpectStatus(c, ackReceived, readBuffer)
}

func (suite *GoBackNArqTestSuite) readExpectStatus(c *extensionDelegator, expected statusCode, readBuffer []byte) {
	status, _, err := c.Read(readBuffer)
	suite.handleTestError(err)
	suite.Equal(expected, status)
}

func (suite *GoBackNArqTestSuite) handleTestError(err error) {
	if err != nil {
		suite.Errorf(err, "Error occurred")
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
	suite.handleTestError(suite.alphaConnection.Open())
	suite.handleTestError(suite.betaConnection.Open())
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
	setSegmentMtu(headerLength + 4)
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
	setSegmentMtu(headerLength + 4)
	suite.alphaManipulator.dropOnce(2)
	message := "testTESTtEsT"
	writeBuffer := []byte(message)
	readBuffer := make([]byte, segmentMtu)
	suite.write(suite.alphaConnection, writeBuffer)
	suite.read(suite.betaConnection, "test", readBuffer)
	suite.readAck(suite.alphaConnection, readBuffer)
	suite.readExpectStatus(suite.betaConnection, invalidSegment, readBuffer)
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
	dataChunkSize = segmentMtu - headerLength
}

func newMockConnection(connector *channelConnector) (*extensionDelegator, *goBackNArq, *segmentManipulator) {
	connection := &extensionDelegator{}
	arq := &goBackNArq{}
	manipulator := &segmentManipulator{}
	connection.addExtension(arq)
	arq.addExtension(manipulator)
	manipulator.addExtension(connector)
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

func (manipulator *segmentManipulator) Write(buffer []byte) (statusCode, int, error) {
	seg := createSegment(buffer)
	for elem := manipulator.toDropOnce.Front(); elem != nil; elem = elem.Next() {
		if elem.Value.(uint32) == seg.getSequenceNumber() {
			manipulator.toDropOnce.Remove(elem)
			return success, len(buffer), nil
		}
	}
	return manipulator.extension.Write(buffer)
}

type channelConnector struct {
	in  chan []byte
	out chan []byte
}

func (connector *channelConnector) Open() error {
	return nil
}

func (connector *channelConnector) Close() error {
	close(connector.in)
	return nil
}

func (connector *channelConnector) Write(buffer []byte) (statusCode, int, error) {
	connector.out <- buffer
	return success, len(buffer), nil
}

func (connector *channelConnector) Read(buffer []byte) (statusCode, int, error) {
	buff := <-connector.in
	copy(buffer, buff)
	return success, len(buff), nil

}
