package protocol

import (
	"github.com/stretchr/testify/suite"
	"testing"
	"time"
)

func newMockSelectiveArqConnection(connector *channelConnector, name string) (*selectiveArq, *segmentManipulator) {
	arq := selectiveArq{}
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

type SelectiveArqTestSuite struct {
	suite.Suite
	alphaArq, betaArq                 *selectiveArq
	alphaManipulator, betaManipulator *segmentManipulator
	sequenceNumberQueue               Queue
}

func (suite *SelectiveArqTestSuite) handleTestError(err error) {
	if err != nil {
		suite.Errorf(err, "Error occurred")
	}
}
func (suite *SelectiveArqTestSuite) SetupTest() {
	endpoint1, endpoint2 := make(chan []byte, 100), make(chan []byte, 100)
	connector1, connector2 := &channelConnector{
		in:  endpoint1,
		out: endpoint2,
	}, &channelConnector{
		in:  endpoint2,
		out: endpoint1,
	}
	suite.alphaArq, suite.alphaManipulator = newMockSelectiveArqConnection(connector1, "alpha")
	suite.betaArq, suite.betaManipulator = newMockSelectiveArqConnection(connector2, "beta")

	suite.sequenceNumberQueue = Queue{}
	suite.sequenceNumberQueue.Enqueue(uint32(1))
	suite.sequenceNumberQueue.Enqueue(uint32(2))

	suite.handleTestError(suite.alphaArq.Open())
	suite.handleTestError(suite.betaArq.Open())
}
func (suite *SelectiveArqTestSuite) TearDownTest() {
	segmentMtu = DefaultMTU
}

func (suite *SelectiveArqTestSuite) write(c Connector, data []byte) {
	status, _, err := c.Write(data)
	suite.handleTestError(err)
	suite.Equal(success, status)
}
func (suite *SelectiveArqTestSuite) read(c Connector, expected string, readBuffer []byte) {
	status, n, err := c.Read(readBuffer)
	suite.handleTestError(err)
	suite.Equal(expected, string(readBuffer[:n]))
	suite.Equal(success, status)
}
func (suite *SelectiveArqTestSuite) readExpectStatus(c Connector, expected statusCode, readBuffer []byte) {
	status, _, err := c.Read(readBuffer)
	suite.handleTestError(err)
	suite.Equal(expected, status)
}
func (suite *SelectiveArqTestSuite) readAck(c Connector, readBuffer []byte) {
	suite.readExpectStatus(c, ackReceived, readBuffer)
}

func (suite *SelectiveArqTestSuite) TestSelectiveAckDropFourOfEight() {
	segmentMtu = HeaderLength + 4

	suite.alphaManipulator.dropOnce(5)
	suite.alphaManipulator.dropOnce(6)
	suite.alphaManipulator.dropOnce(7)

	RetransmissionTimeout = 20 * time.Millisecond
	message := "ABCDEFGHIJKLMNOPQRSTUVWXYZ123456"
	writeBuffer := []byte(message)
	readBuffer := make([]byte, segmentMtu)
	suite.write(suite.alphaArq, writeBuffer)
	suite.read(suite.betaArq, "ABCD", readBuffer)
	suite.readAck(suite.alphaArq, readBuffer)
	suite.read(suite.betaArq, "EFGH", readBuffer)
	suite.readAck(suite.alphaArq, readBuffer)
	suite.read(suite.betaArq, "IJKL", readBuffer)
	suite.readAck(suite.alphaArq, readBuffer)
	suite.read(suite.betaArq, "MNOP", readBuffer)
	suite.readAck(suite.alphaArq, readBuffer)
	suite.readExpectStatus(suite.betaArq, 4, readBuffer)
	suite.readAck(suite.alphaArq, readBuffer)
	suite.read(suite.betaArq, "QRST", readBuffer)
	suite.readAck(suite.alphaArq, readBuffer)
	suite.read(suite.betaArq, "UVWX", readBuffer)
	suite.readAck(suite.alphaArq, readBuffer)
	suite.read(suite.betaArq, "YZ12", readBuffer)
	suite.readAck(suite.alphaArq, readBuffer)
	suite.read(suite.betaArq, "3456", readBuffer)
	suite.readAck(suite.alphaArq, readBuffer)
}
func (suite *SelectiveArqTestSuite) TestSendInOneSegmentSelectiveACK() {
	message := "Hello, World!"
	writeBuffer := []byte(message)
	readBuffer := make([]byte, segmentMtu)

	alpha := selectiveArq{}
	alpha.addExtension(suite.alphaArq.extension)
	beta := selectiveArq{}
	beta.addExtension(suite.betaArq.extension)
	suite.alphaArq = &alpha
	suite.betaArq = &beta
	suite.write(suite.alphaArq, writeBuffer)
	suite.read(suite.betaArq, message, readBuffer)
	suite.readAck(suite.alphaArq, readBuffer)
	suite.Equal(uint32(1), suite.alphaArq.lastAckedSegmentSequenceNumber)
}

func TestSelectiveArq(t *testing.T) {
	suite.Run(t, new(SelectiveArqTestSuite))
}
