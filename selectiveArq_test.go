package atp

import (
	"github.com/stretchr/testify/suite"
	"strings"
	"testing"
	"time"
)

func repeatDataSize(s int, n int) string {
	str := ""
	for i := 0; i < n; i++ {
		str += strings.Repeat(string(s+i), getDataChunkSize())
	}
	return str
}

type ArqTestSuite struct {
	atpTestSuite
	alphaArq, betaArq                 *selectiveArq
	alphaManipulator, betaManipulator *segmentManipulator
}

func newMockSelectiveRepeatArqConnection(connector *channelConnector, name string) (*selectiveArq, *segmentManipulator) {
	manipulator := &segmentManipulator{extension: connector, toDropOnce: make([]uint32, 0)}
	arq := newSelectiveArq(1, manipulator, testErrorChannel)
	return arq, manipulator
}

func (suite *ArqTestSuite) SetupTest() {
	suite.timestamp = time.Now()
	endpoint1, endpoint2 := make(chan []byte, 100), make(chan []byte, 100)
	connection1, connection2 := &channelConnector{
		in:            endpoint1,
		out:           endpoint2,
		artificialNow: suite.timestamp,
	}, &channelConnector{
		in:            endpoint2,
		out:           endpoint1,
		artificialNow: suite.timestamp,
	}
	suite.alphaArq, suite.alphaManipulator = newMockSelectiveRepeatArqConnection(connection1, "alpha")
	suite.betaArq, suite.betaManipulator = newMockSelectiveRepeatArqConnection(connection2, "beta")
	segmentMtu = headerLength + 32
}

func (suite *ArqTestSuite) TearDownTest() {
	segmentMtu = defaultMTU
	suite.handleTestError(suite.alphaArq.Close())
	suite.handleTestError(suite.betaArq.Close())
}

func (suite *ArqTestSuite) TestSimpleWrite() {
	suite.betaArq.ackThreshold = 1
	suite.write(suite.alphaArq, repeatDataSize('A', 1), suite.timestamp)
	suite.read(suite.betaArq, repeatDataSize('A', 1), suite.timestamp)
	suite.readAck(suite.alphaArq, suite.timestamp)
	suite.readExpectStatus(suite.alphaArq, timeout, suite.timeout())
}

func (suite *ArqTestSuite) TestWriteTwoSegments() {
	suite.betaArq.ackThreshold = 2
	suite.write(suite.alphaArq, repeatDataSize('A', 2), suite.timestamp)
	suite.read(suite.betaArq, repeatDataSize('A', 1), suite.timestamp)
	suite.read(suite.betaArq, repeatDataSize('B', 1), suite.timestamp)
	suite.readAck(suite.alphaArq, suite.timestamp)
	suite.Empty(suite.alphaArq.waitingForAck)
	suite.readExpectStatus(suite.alphaArq, timeout, suite.timeout())
}

func (suite *ArqTestSuite) TestWriteAckAfterTimeout() {
	suite.betaArq.ackThreshold = 3
	suite.write(suite.alphaArq, repeatDataSize('A', 2), suite.timestamp)
	suite.read(suite.betaArq, repeatDataSize('A', 1), suite.timestamp)
	suite.read(suite.betaArq, repeatDataSize('B', 1), suite.timestamp)
	suite.readExpectStatus(suite.betaArq, timeout, suite.timestamp)
	suite.readAck(suite.alphaArq, suite.timestamp)
	suite.Empty(suite.alphaArq.waitingForAck)
	suite.readExpectStatus(suite.alphaArq, timeout, suite.timeout())
}

func (suite *ArqTestSuite) TestRetransmitLostSegmentsOnAck() {
	suite.betaArq.ackThreshold = 3
	suite.alphaManipulator.DropOnce(2)
	suite.alphaManipulator.DropOnce(3)
	suite.write(suite.alphaArq, repeatDataSize('A', 5), suite.timestamp)
	suite.read(suite.betaArq, repeatDataSize('A', 1), suite.timestamp)
	suite.readExpectStatus(suite.betaArq, invalidSegment, suite.timestamp)
	suite.readExpectStatus(suite.betaArq, invalidSegment, suite.timestamp)

	suite.readAck(suite.alphaArq, suite.timestamp)
	suite.write(suite.alphaArq, "", suite.timestamp)
	suite.read(suite.betaArq, repeatDataSize('B', 1), suite.timestamp)
	suite.read(suite.betaArq, repeatDataSize('C', 1), suite.timestamp)
	suite.read(suite.betaArq, repeatDataSize('D', 1), suite.timestamp)
	suite.read(suite.betaArq, repeatDataSize('E', 1), suite.timestamp)
	suite.readAck(suite.alphaArq, suite.timestamp)

	suite.Empty(suite.alphaArq.waitingForAck)
	suite.readExpectStatus(suite.alphaArq, timeout, suite.timeout())
}

func TestSelectiveRepeatArq(t *testing.T) {
	suite.Run(t, new(ArqTestSuite))
}
