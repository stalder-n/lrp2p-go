package atp

import (
	"github.com/stretchr/testify/suite"
	"testing"
	"time"
)

type ArqTestSuite struct {
	atpTestSuite
	alphaArq, betaArq                 *selectiveArq
	alphaManipulator, betaManipulator *segmentManipulator
	alphaConn, betaConn               *Conn
}

func newMockSelectiveRepeatArqConnection(connector *channelConnector) (*selectiveArq, *segmentManipulator) {
	manipulator := &segmentManipulator{conn: connector, toDropOnce: make([]uint32, 0)}
	arq := newSelectiveArq(manipulator.Write, testErrorChannel)
	return arq, manipulator
}

func (suite *ArqTestSuite) writeExpectStatus(c *Conn, payload string, code statusCode, timestamp time.Time) {
	c.arq.queueNewSegments([]byte(payload))
	status, err := c.arq.writeQueuedSegments(timestamp)
	suite.handleTestError(err)
	suite.Equal(code, status)
}

func (suite *ArqTestSuite) write(c *Conn, payload string, timestamp time.Time) {
	suite.writeExpectStatus(c, payload, success, timestamp)
}

func (suite *ArqTestSuite) read(c *Conn, expected string, timestamp time.Time) {
	readBuffer := make([]byte, segmentMtu)
	status, n, err := c.readFromEndpoint(readBuffer, timestamp)
	suite.handleTestError(err)
	status, segs := c.arq.processReceivedSegment(readBuffer[:n], false, timestamp)
	suite.Len(segs, 1)
	seg := segs[0]
	suite.handleTestError(err)
	suite.Equal(success, status)
	suite.Equal(expected, string(seg.data))
}

func (suite *ArqTestSuite) readExpectStatus(c *Conn, expected statusCode, timestamp time.Time) {
	readBuffer := make([]byte, segmentMtu)
	status, n, err := c.readFromEndpoint(readBuffer, timestamp)
	suite.handleTestError(err)
	if status != success {
		suite.Equal(expected, status)
		return
	}
	status, _ = c.arq.processReceivedSegment(readBuffer[:n], false, timestamp)
	suite.handleTestError(err)
	suite.Equal(expected, status)
}

func (suite *ArqTestSuite) readAck(c *Conn, timestamp time.Time) {
	suite.readExpectStatus(c, ackReceived, timestamp)
}

func (suite *ArqTestSuite) SetupTest() {
	suite.timestamp = time.Now()
	endpoint1, endpoint2 := make(chan []byte, 100), make(chan []byte, 100)
	connection1, connection2 := &channelConnector{
		in:            endpoint1,
		out:           endpoint2,
		artificialNow: suite.timestamp,
		timeout:       1 * time.Millisecond,
	}, &channelConnector{
		in:            endpoint2,
		out:           endpoint1,
		artificialNow: suite.timestamp,
		timeout:       1 * time.Millisecond,
	}
	suite.alphaArq, suite.alphaManipulator = newMockSelectiveRepeatArqConnection(connection1)
	suite.betaArq, suite.betaManipulator = newMockSelectiveRepeatArqConnection(connection2)
	suite.alphaArq.cwnd = 10
	suite.betaArq.cwnd = 10

	suite.alphaConn = newConn(0, suite.alphaManipulator)
	suite.alphaConn.arq = suite.alphaArq
	suite.betaConn = newConn(1, suite.betaManipulator)
	suite.betaConn.arq = suite.betaArq
}

func (suite *ArqTestSuite) TearDownTest() {
	suite.handleTestError(suite.alphaManipulator.conn.Close())
	suite.handleTestError(suite.betaManipulator.conn.Close())
}

func (suite *ArqTestSuite) TestSimpleWrite() {
	suite.write(suite.alphaConn, repeatDataSize('A', 1), suite.timestamp)
	suite.read(suite.betaConn, repeatDataSize('A', 1), suite.timestamp)
	suite.readAck(suite.alphaConn, suite.timestamp)
}

func (suite *ArqTestSuite) TestWriteTwoSegments() {
	suite.write(suite.alphaConn, repeatDataSizeInc('A', 2), suite.timestamp)
	suite.read(suite.betaConn, repeatDataSize('A', 1), suite.timestamp)
	suite.read(suite.betaConn, repeatDataSize('B', 1), suite.timestamp)
	suite.readAck(suite.alphaConn, suite.timestamp)
	suite.readAck(suite.alphaConn, suite.timestamp)
	suite.True(suite.alphaArq.waitingForAck.isEmpty())
}

func (suite *ArqTestSuite) TestRetransmitLostSegmentOnAck() {
	suite.alphaManipulator.DropOnce(1)
	suite.write(suite.alphaConn, repeatDataSizeInc('A', 5), suite.timestamp)
	suite.read(suite.betaConn, repeatDataSizeInc('A', 1), suite.timestamp)
	suite.readAck(suite.alphaConn, suite.timestamp)

	suite.readExpectStatus(suite.betaConn, invalidSegment, suite.timestamp)
	suite.readAck(suite.alphaConn, suite.timestamp)

	suite.readExpectStatus(suite.betaConn, invalidSegment, suite.timestamp)
	suite.readAck(suite.alphaConn, suite.timestamp)

	suite.readExpectStatus(suite.betaConn, invalidSegment, suite.timestamp)
	suite.readAck(suite.alphaConn, suite.timestamp)

	suite.read(suite.betaConn, repeatDataSize('B', 1), suite.timestamp)
	suite.readAck(suite.alphaConn, suite.timestamp)
	suite.read(suite.betaConn, repeatDataSize('C', 1), suite.timestamp)
	suite.read(suite.betaConn, repeatDataSize('D', 1), suite.timestamp)
	suite.read(suite.betaConn, repeatDataSize('E', 1), suite.timestamp)
}

func (suite *ArqTestSuite) TestRetransmitLostSegmentsOnTimeout() {
	suite.alphaManipulator.DropOnce(1)
	suite.write(suite.alphaConn, repeatDataSizeInc('A', 2), suite.timestamp)
	suite.read(suite.betaConn, repeatDataSize('A', 1), suite.timestamp)
	suite.readAck(suite.alphaConn, suite.timestamp)
	suite.Equal(uint32(1), suite.alphaConn.arq.waitingForAck.numOfSegments())

	suite.alphaConn.arq.retransmitTimedOutSegments(suite.timestamp.Add(suite.alphaConn.arq.rto + 1))

	suite.read(suite.betaConn, repeatDataSize('B', 1), suite.timestamp)
	suite.readAck(suite.alphaConn, suite.timestamp)
	suite.True(suite.alphaArq.waitingForAck.isEmpty())
}

func (suite *ArqTestSuite) TestMeasureRTOWithSteadyRTT() {
	suite.alphaArq.cwnd = 10
	suite.betaArq.cwnd = 10
	rttTimestamp := suite.timestamp.Add(100 * time.Millisecond)
	suite.write(suite.alphaConn, repeatDataSizeInc('A', 5), suite.timestamp)
	suite.Equal(5, suite.alphaArq.rttToMeasure)
	suite.read(suite.betaConn, repeatDataSize('A', 1), suite.timestamp)
	suite.read(suite.betaConn, repeatDataSize('B', 1), suite.timestamp)
	suite.read(suite.betaConn, repeatDataSize('C', 1), suite.timestamp)
	suite.read(suite.betaConn, repeatDataSize('D', 1), suite.timestamp)
	suite.read(suite.betaConn, repeatDataSize('E', 1), suite.timestamp)

	suite.readAck(suite.alphaConn, rttTimestamp)
	suite.assertRTTValues(suite.alphaArq, 100, 50, 300)

	suite.readAck(suite.alphaConn, rttTimestamp)
	suite.assertRTTValues(suite.alphaArq, 100, 37.5, 250)

	suite.readAck(suite.alphaConn, rttTimestamp)
	suite.assertRTTValues(suite.alphaArq, 100, 28.125, 212.5)

	suite.readAck(suite.alphaConn, rttTimestamp)
	suite.assertRTTValues(suite.alphaArq, 100, 0, 200)

	suite.readAck(suite.alphaConn, rttTimestamp)
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
