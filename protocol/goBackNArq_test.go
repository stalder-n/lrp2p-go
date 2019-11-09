package protocol

import (
	"github.com/stretchr/testify/suite"
	"testing"
	"time"
)

func newMockConnection(connector *channelConnector, name string) (*goBackNArq, *segmentManipulator) {
	arq := goBackNArq{}
	printer := &consolePrinter{name: name}
	manipulator := &segmentManipulator{}

	arq.addExtension(printer)
	printer.addExtension(manipulator)
	manipulator.addExtension(connector)

	fu := func() uint32 {
		return 1
	}

	arq.sequenceNumberFactory = fu

	return &arq, manipulator
}

type GoBackNArqTestSuite struct {
	suite.Suite
	alphaArq, betaArq                 *goBackNArq
	alphaManipulator, betaManipulator *segmentManipulator
	sequenceNumberQueue               Queue
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
	suite.alphaArq, suite.alphaManipulator = newMockConnection(connector1, "alpha")
	suite.betaArq, suite.betaManipulator = newMockConnection(connector2, "beta")

	suite.sequenceNumberQueue = Queue{}
	suite.sequenceNumberQueue.Enqueue(uint32(1))
	suite.sequenceNumberQueue.Enqueue(uint32(2))

	suite.handleTestError(suite.alphaArq.Open())
	suite.handleTestError(suite.betaArq.Open())
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
	message := "Hello, World!"
	writeBuffer := []byte(message)
	readBuffer := make([]byte, segmentMtu)
	suite.write(suite.alphaArq, writeBuffer)
	suite.read(suite.betaArq, message, readBuffer)
	suite.readAck(suite.alphaArq, readBuffer)
	suite.Equal(uint32(1), suite.alphaArq.lastAckedSegmentSequenceNumber)
}
func (suite *GoBackNArqTestSuite) TestRetransmissionByTimeout() {
	suite.alphaManipulator.dropOnce(1)
	RetransmissionTimeout = 20 * time.Millisecond
	message := "Hello, World!"
	writeBuffer := []byte(message)
	readBuffer := make([]byte, segmentMtu)
	suite.write(suite.alphaArq, writeBuffer)
	time.Sleep(RetransmissionTimeout)
	suite.alphaArq.writeMissingSegment()
	suite.read(suite.betaArq, message, readBuffer)
	suite.readAck(suite.alphaArq, readBuffer)
	suite.Equal(uint32(1), suite.alphaArq.lastAckedSegmentSequenceNumber)
}
func (suite *GoBackNArqTestSuite) TestSendSegmentsInOrder() {
	suite.sequenceNumberQueue.Enqueue(uint32(1))
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
	suite.Equal(uint32(3), suite.alphaArq.lastAckedSegmentSequenceNumber)
}
func (suite *GoBackNArqTestSuite) TestSendSegmentsOutOfOrder() {
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
	suite.read(suite.betaArq, "TEST", readBuffer)
	suite.readAck(suite.alphaArq, readBuffer)
	suite.read(suite.betaArq, "tEsT", readBuffer)
	suite.readAck(suite.alphaArq, readBuffer)
	suite.Equal(uint32(3), suite.alphaArq.lastAckedSegmentSequenceNumber)
}

func TestGoBackNArq(t *testing.T) {
	suite.Run(t, new(GoBackNArqTestSuite))
}
