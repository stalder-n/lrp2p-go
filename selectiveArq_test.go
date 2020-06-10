package atp

import (
	"github.com/stretchr/testify/suite"
	"strings"
	"testing"
	"time"
)

func repeatDataSizeInc(s int, n int) string {
	str := ""
	for i := 0; i < n; i++ {
		str += strings.Repeat(string(s+i), getDataChunkSize())
	}
	return str
}

func repeatDataSize(s int, n int) string {
	return strings.Repeat(string(s), n*getDataChunkSize())
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
	suite.alphaArq, suite.alphaManipulator = newMockSelectiveRepeatArqConnection(connection1, "rttAlpha")
	suite.betaArq, suite.betaManipulator = newMockSelectiveRepeatArqConnection(connection2, "rttBeta")
	suite.alphaArq.cwnd = 10
	suite.betaArq.cwnd = 10
}

func (suite *ArqTestSuite) TearDownTest() {
	suite.handleTestError(suite.alphaArq.Close())
	suite.handleTestError(suite.betaArq.Close())
}

func (suite *ArqTestSuite) TestSimpleWrite() {
	suite.write(suite.alphaArq, repeatDataSizeInc('A', 1), suite.timestamp)
	suite.read(suite.betaArq, repeatDataSizeInc('A', 1), suite.timestamp)
	suite.readAck(suite.alphaArq, suite.timestamp)
}

func (suite *ArqTestSuite) TestWriteTwoSegments() {
	suite.write(suite.alphaArq, repeatDataSizeInc('A', 2), suite.timestamp)
	suite.read(suite.betaArq, repeatDataSizeInc('A', 1), suite.timestamp)
	suite.read(suite.betaArq, repeatDataSizeInc('B', 1), suite.timestamp)
	suite.readAck(suite.alphaArq, suite.timestamp)
	suite.readAck(suite.alphaArq, suite.timestamp)
	suite.Empty(suite.alphaArq.waitingForAck)
}

func (suite *ArqTestSuite) TestWriteAckAfterTimeout() {
	suite.write(suite.alphaArq, repeatDataSizeInc('A', 2), suite.timestamp)
	suite.read(suite.betaArq, repeatDataSizeInc('A', 1), suite.timestamp)
	suite.read(suite.betaArq, repeatDataSizeInc('B', 1), suite.timestamp)
	suite.readExpectStatus(suite.betaArq, timeout, suite.timestamp.Add(defaultArqTimeout))
	suite.readAck(suite.alphaArq, suite.timestamp)
	suite.readAck(suite.alphaArq, suite.timestamp)
	suite.Empty(suite.alphaArq.waitingForAck)
}

func (suite *ArqTestSuite) TestRetransmitLostSegmentOnAck() {
	suite.alphaManipulator.DropOnce(2)
	suite.write(suite.alphaArq, repeatDataSizeInc('A', 5), suite.timestamp)
	suite.read(suite.betaArq, repeatDataSizeInc('A', 1), suite.timestamp)
	suite.readAck(suite.alphaArq, suite.timestamp)

	suite.readExpectStatus(suite.betaArq, invalidSegment, suite.timestamp)
	suite.readAck(suite.alphaArq, suite.timestamp)

	suite.readExpectStatus(suite.betaArq, invalidSegment, suite.timestamp)
	suite.readAck(suite.alphaArq, suite.timestamp)

	suite.readExpectStatus(suite.betaArq, invalidSegment, suite.timestamp)
	suite.readAck(suite.alphaArq, suite.timestamp)

	suite.read(suite.betaArq, repeatDataSizeInc('B', 1), suite.timestamp)
	suite.readAck(suite.alphaArq, suite.timestamp)
	suite.read(suite.betaArq, repeatDataSizeInc('C', 1), suite.timestamp)
	suite.read(suite.betaArq, repeatDataSizeInc('D', 1), suite.timestamp)
	suite.read(suite.betaArq, repeatDataSizeInc('E', 1), suite.timestamp)

	suite.Empty(suite.alphaArq.waitingForAck)
	suite.Empty(suite.betaArq.segmentBuffer)
}

func (suite *ArqTestSuite) TestRetransmitLostSegmentsOnTimeout() {
	suite.alphaManipulator.DropOnce(2)
	suite.write(suite.alphaArq, repeatDataSizeInc('A', 2), suite.timestamp)
	suite.read(suite.betaArq, repeatDataSizeInc('A', 1), suite.timestamp)
	suite.readExpectStatus(suite.betaArq, timeout, suite.timestamp)
	suite.readAck(suite.alphaArq, suite.timestamp)
	suite.Len(suite.alphaArq.waitingForAck, 1)

	suite.write(suite.alphaArq, "", suite.timestamp.Add(suite.alphaArq.rto+1))
	suite.read(suite.betaArq, repeatDataSizeInc('B', 1), suite.timestamp)
	suite.readAck(suite.alphaArq, suite.timestamp)

	suite.Empty(suite.alphaArq.waitingForAck)
}

func (suite *ArqTestSuite) TestMeasureRTOWithSteadyRTT() {
	suite.alphaArq.cwnd = 10
	suite.betaArq.cwnd = 10
	rttTimestamp := suite.timestamp.Add(100 * time.Millisecond)
	suite.write(suite.alphaArq, repeatDataSizeInc('A', 5), suite.timestamp)
	suite.Equal(5, suite.alphaArq.rttToMeasure)
	suite.read(suite.betaArq, repeatDataSizeInc('A', 1), suite.timestamp)
	suite.read(suite.betaArq, repeatDataSizeInc('B', 1), suite.timestamp)
	suite.read(suite.betaArq, repeatDataSizeInc('C', 1), suite.timestamp)
	suite.read(suite.betaArq, repeatDataSizeInc('D', 1), suite.timestamp)
	suite.read(suite.betaArq, repeatDataSizeInc('E', 1), suite.timestamp)

	suite.readAck(suite.alphaArq, rttTimestamp)
	suite.assertRTTValues(suite.alphaArq, 100, 50, 300)

	suite.readAck(suite.alphaArq, rttTimestamp)
	suite.assertRTTValues(suite.alphaArq, 100, 37.5, 250)

	suite.readAck(suite.alphaArq, rttTimestamp)
	suite.assertRTTValues(suite.alphaArq, 100, 28.125, 212.5)

	suite.readAck(suite.alphaArq, rttTimestamp)
	suite.assertRTTValues(suite.alphaArq, 100, 0, 200)

	suite.readAck(suite.alphaArq, rttTimestamp)
	suite.assertRTTValues(suite.alphaArq, 100, 0, 200)

	suite.Equal(0, suite.alphaArq.rttToMeasure)
}

func (suite *ArqTestSuite) assertRTTValues(arq *selectiveArq, sRtt, rttVar float64, rto float64) {
	suite.Equal(sRtt*1e6, arq.sRtt)
	if rttVar > 0 {
		suite.Equal(rttVar*1e6, arq.rttVar)
	}
	suite.Equal(time.Duration(rto*1e6), arq.rto)
}

func TestSelectiveRepeatArq(t *testing.T) {
	suite.Run(t, new(ArqTestSuite))
}
