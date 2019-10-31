package protocol

import (
	"container/list"
	"github.com/stretchr/testify/suite"
	"testing"
	"time"
)

func newMockConnection(connector *channelConnector) (*goBackNArq, *segmentManipulator) {
	arq := &goBackNArq{}
	manipulator := &segmentManipulator{}
	arq.addExtension(manipulator)
	manipulator.addExtension(connector)
	return arq, manipulator
}

type segmentManipulator struct {
	savedSegments map[uint32][]byte
	toDropOnce    list.List
	extension     Connector
}

func (manipulator *segmentManipulator) Read(buffer []byte) (statusCode, int, error) {
	return manipulator.extension.Read(buffer)
}
func (manipulator *segmentManipulator) Open() error {
	return manipulator.extension.Open()
}
func (manipulator *segmentManipulator) Close() error {
	return manipulator.extension.Close()
}
func (manipulator *segmentManipulator) addExtension(connector Connector) {
	manipulator.extension = connector
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

type GoBackNArqTestSuite struct {
	suite.Suite
	alphaArq, betaArq                 *goBackNArq
	alphaManipulator, betaManipulator *segmentManipulator
	initialSequenceNumberQueue        queue
}

func (suite *GoBackNArqTestSuite) handleTestError(err error) {
	if err != nil {
		suite.Errorf(err, "Error occurred")
	}
}
func (suite *GoBackNArqTestSuite) SetupTest() {
	endpoint1, endpoint2 := make(chan []byte, 100), make(chan []byte, 100)
	connector1, connector2 := &channelConnector{
		in:  endpoint1,
		out: endpoint2,
	}, &channelConnector{
		in:  endpoint2,
		out: endpoint1,
	}
	suite.alphaArq, suite.alphaManipulator = newMockConnection(connector1)
	suite.betaArq, suite.betaManipulator = newMockConnection(connector2)

	suite.handleTestError(suite.alphaArq.Open())
	suite.handleTestError(suite.betaArq.Open())

	suite.initialSequenceNumberQueue = queue{}
	sequenceNumberFactory = func() uint32 {
		return suite.initialSequenceNumberQueue.Dequeue().(uint32)
	}
}
func (suite *GoBackNArqTestSuite) TearDownTest() {
	segmentMtu = DefaultMTU
}

func (suite *GoBackNArqTestSuite) write(c Connector, data []byte) {
	status, _, err := c.Write(data)
	suite.handleTestError(err)
	suite.Equal(success, status)
}
func (suite *GoBackNArqTestSuite) read(c Connector, expected string, readBuffer []byte) {
	status, n, err := c.Read(readBuffer)
	suite.handleTestError(err)
	suite.Equal(expected, string(readBuffer[:n]))
	suite.Equal(success, status)
}
func (suite *GoBackNArqTestSuite) readExpectStatus(c Connector, expected statusCode, readBuffer []byte) {
	status, _, err := c.Read(readBuffer)
	suite.handleTestError(err)
	suite.Equal(expected, status)
}
func (suite *GoBackNArqTestSuite) readAck(c Connector, readBuffer []byte) {
	suite.readExpectStatus(c, ackReceived, readBuffer)
}

func (suite *GoBackNArqTestSuite) TestSendInOneSegment() {
	suite.initialSequenceNumberQueue.Enqueue(uint32(1))
	message := "Hello, World!"
	writeBuffer := []byte(message)
	readBuffer := make([]byte, segmentMtu)
	suite.write(suite.alphaArq, writeBuffer)
	suite.read(suite.betaArq, message, readBuffer)
	suite.readAck(suite.alphaArq, readBuffer)
	suite.Equal(0, suite.alphaArq.lastAckedSequenceNumber)
}
func (suite *GoBackNArqTestSuite) TestRetransmissionByTimeout() {
	suite.initialSequenceNumberQueue.Enqueue(uint32(1))
	suite.alphaManipulator.dropOnce(1)
	RetransmissionTimeout = 20 * time.Millisecond
	message := "Hello, World!"
	writeBuffer := []byte(message)
	readBuffer := make([]byte, segmentMtu)
	suite.write(suite.alphaArq, writeBuffer)
	time.Sleep(RetransmissionTimeout)
	suite.alphaArq.WriteTimedOutSegments()
	suite.read(suite.betaArq, message, readBuffer)
	suite.readAck(suite.alphaArq, readBuffer)
	suite.Equal(0, suite.alphaArq.lastAckedSequenceNumber)
}
func (suite *GoBackNArqTestSuite) TestSendSegmentsInOrder() {
	suite.initialSequenceNumberQueue.Enqueue(uint32(1))
	segmentMtu = HeaderLength + 4
	message := "testTESTtEsT"
	writeBuffer := []byte(message)
	readBuffer := make([]byte, segmentMtu)
	suite.write(suite.alphaArq, writeBuffer)
	suite.read(suite.betaArq, "test", readBuffer)
	suite.readAck(suite.alphaArq, readBuffer)
	suite.read(suite.betaArq, "TEST", readBuffer)
	suite.readAck(suite.alphaArq, readBuffer)
	suite.read(suite.betaArq, "tEsT", readBuffer)
	suite.readAck(suite.alphaArq, readBuffer)
	suite.Equal(2, suite.alphaArq.lastAckedSequenceNumber)
}
func (suite *GoBackNArqTestSuite) TestSendSegmentsOutOfOrder() {
	suite.initialSequenceNumberQueue.Enqueue(uint32(1))
	segmentMtu = HeaderLength + 4
	suite.alphaManipulator.dropOnce(2)
	message := "testTESTtEsT"
	writeBuffer := []byte(message)
	readBuffer := make([]byte, segmentMtu)
	suite.write(suite.alphaArq, writeBuffer)
	suite.read(suite.betaArq, "test", readBuffer)
	suite.readAck(suite.alphaArq, readBuffer)
	suite.readExpectStatus(suite.betaArq, invalidSegment, readBuffer)
	suite.readAck(suite.alphaArq, readBuffer)
	suite.alphaArq.WriteTimedOutSegments()
	suite.read(suite.betaArq, "TEST", readBuffer)
	suite.readAck(suite.alphaArq, readBuffer)
	suite.read(suite.betaArq, "tEsT", readBuffer)
	suite.readAck(suite.alphaArq, readBuffer)
	suite.Equal(2, suite.alphaArq.lastAckedSequenceNumber)
}

func TestGoBackNArq(t *testing.T) {
	suite.Run(t, new(GoBackNArqTestSuite))
}
