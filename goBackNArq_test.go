package atp

import (
	"github.com/stretchr/testify/suite"
	"testing"
	"time"
)

func newMockConnection(connector *channelConnector, name string) (*goBackNArq, *segmentManipulator) {
	arq := goBackNArq{}
	printer := &consolePrinter{Name: name}
	manipulator := &segmentManipulator{}

	arq.addExtension(printer)
	printer.AddExtension(manipulator)
	manipulator.AddExtension(connector)

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
	sequenceNumberQueue               *queue
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

	suite.sequenceNumberQueue = newQueue()
	suite.sequenceNumberQueue.Enqueue(uint32(1))
	suite.sequenceNumberQueue.Enqueue(uint32(2))

	suite.handleTestError(suite.alphaArq.Open())
	suite.handleTestError(suite.betaArq.Open())
}
func (suite *GoBackNArqTestSuite) TearDownTest() {
	segmentMtu = defaultMTU
}

func (suite *GoBackNArqTestSuite) write(c Connector, data []byte) {
	status, _, err := c.Write(data, time.Now())
	suite.handleTestError(err)
	suite.Equal(success, status)
}
func (suite *GoBackNArqTestSuite) read(c Connector, expected string, readBuffer []byte) {
	status, n, err := c.Read(readBuffer, time.Now())
	suite.handleTestError(err)
	suite.Equal(expected, string(readBuffer[:n]))
	suite.Equal(success, status)
}
func (suite *GoBackNArqTestSuite) readExpectStatus(c Connector, expected statusCode, readBuffer []byte) {
	status, _, err := c.Read(readBuffer, time.Now())
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
	suite.Equal(uint32(1), suite.alphaArq.lastInOrderNumber)
}
func (suite *GoBackNArqTestSuite) TestRetransmissionByTimeout() {
	suite.alphaManipulator.DropOnce(1)
	retransmissionTimeout = 20 * time.Millisecond
	message := "Hello, World!"
	writeBuffer := []byte(message)
	readBuffer := make([]byte, segmentMtu)
	suite.write(suite.alphaArq, writeBuffer)
	time.Sleep(retransmissionTimeout)
	suite.alphaArq.writeMissingSegment(time.Now())
	suite.read(suite.betaArq, message, readBuffer)
	suite.readAck(suite.alphaArq, readBuffer)
	suite.Equal(uint32(1), suite.alphaArq.lastInOrderNumber)
}
func (suite *GoBackNArqTestSuite) TestSendSegmentsInOrder() {
	suite.sequenceNumberQueue.Enqueue(uint32(1))
	segmentMtu = headerLength + 4
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
	suite.Equal(uint32(3), suite.alphaArq.lastInOrderNumber)
}
func (suite *GoBackNArqTestSuite) TestSendSegmentsOutOfOrder() {
	segmentMtu = headerLength + 4
	suite.alphaManipulator.DropOnce(2)
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
	suite.Equal(uint32(3), suite.alphaArq.lastInOrderNumber)
}

func TestGoBackNArq(t *testing.T) {
	suite.Run(t, new(GoBackNArqTestSuite))
}
