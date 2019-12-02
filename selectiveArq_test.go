package atp

import (
	"github.com/stretchr/testify/suite"
	"testing"
	"time"
)

func newMockSelectiveArqConnection(connector *channelConnector, name string) (*selectiveArq, *segmentManipulator) {
	arq := selectiveArq{}
	printer := &consolePrinter{Name: name}
	manipulator := &segmentManipulator{}

	arq.addExtension(manipulator)
	manipulator.AddExtension(printer)
	printer.AddExtension(connector)

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

	suite.handleTestError(suite.alphaArq.Open())
	suite.handleTestError(suite.betaArq.Open())
}
func (suite *SelectiveArqTestSuite) TearDownTest() {
	segmentMtu = defaultMTU
}

func (suite *SelectiveArqTestSuite) write(c Connector, data []byte, time time.Time) {
	status, _, err := c.Write(data, time)
	suite.handleTestError(err)
	suite.Equal(success, status)
}
func (suite *SelectiveArqTestSuite) read(c Connector, expected string, readBuffer []byte, time time.Time) {
	status, n, err := c.Read(readBuffer, time)
	suite.handleTestError(err)
	suite.Equal(expected, string(readBuffer[:n]))
	suite.Equal(success, status)
}
func (suite *SelectiveArqTestSuite) readExpectStatus(c Connector, expected statusCode, readBuffer []byte, time time.Time) {
	status, _, err := c.Read(readBuffer, time)
	suite.handleTestError(err)
	suite.Equal(expected, status)
}
func (suite *SelectiveArqTestSuite) readAck(c Connector, readBuffer []byte, time time.Time) {
	suite.readExpectStatus(c, ackReceived, readBuffer, time)
}

func (suite *SelectiveArqTestSuite) TestQueueTimedOutSegmentsForWrite() {
	arq := selectiveArq{}
	arq.Open()

	currentTime := time.Now()

	arq.notAckedSegment = append(arq.notAckedSegment, &segment{timestamp: currentTime.Add(-1), sequenceNumber: []byte{0, 0, 0, 1}})
	arq.notAckedSegment = append(arq.notAckedSegment, &segment{timestamp: currentTime.Add(-1), sequenceNumber: []byte{0, 0, 0, 2}})
	arq.notAckedSegment = append(arq.notAckedSegment, &segment{timestamp: currentTime.Add(-1), sequenceNumber: []byte{0, 0, 0, 3}})
	arq.notAckedSegment = append(arq.notAckedSegment, &segment{timestamp: currentTime.Add(20000), sequenceNumber: []byte{0, 0, 0, 4}})
	arq.queueTimedOutSegmentsForWrite(currentTime)

	i := uint32(1)
	for !arq.readyToSendSegmentQueue.IsEmpty() {
		ele := arq.readyToSendSegmentQueue.Dequeue()
		suite.Equal(i, ele.(*segment).getSequenceNumber())
		suite.NotEqual(4, ele.(*segment).getSequenceNumber())
		i++
	}
}
func (suite *SelectiveArqTestSuite) TestWriteQueuedSegments() {

	currentTime := time.Now()

	seg1 := createFlaggedSegment(1, 0, []byte("test"))
	seg1.timestamp = currentTime.Add(-1)
	seg2 := createFlaggedSegment(2, 0, []byte("test"))
	seg2.timestamp = currentTime.Add(-1)
	seg3 := createFlaggedSegment(3, 0, []byte("test"))
	seg3.timestamp = currentTime.Add(-1)
	seg4 := createFlaggedSegment(4, 0, []byte("test"))
	seg4.timestamp = currentTime.Add(20000)

	suite.alphaArq.readyToSendSegmentQueue.Enqueue(seg1)
	suite.alphaArq.readyToSendSegmentQueue.Enqueue(seg2)
	suite.alphaArq.readyToSendSegmentQueue.Enqueue(seg3)
	suite.alphaArq.readyToSendSegmentQueue.Enqueue(seg4)

	_, _, err := suite.alphaArq.writeQueuedSegments(currentTime)

	suite.True(suite.alphaArq.readyToSendSegmentQueue.IsEmpty())
	suite.Nil(err)
}

func (suite *SelectiveArqTestSuite) TestFullWindowFlag() {
	segmentMtu = headerLength + 4
	suite.alphaArq.windowSize = 8
	suite.alphaArq.Open()
	suite.betaArq.Open()

	message := "ABCDEFGHIJKLMNOPQRSTUVWXYZ123456"
	writeBuffer := []byte(message)

	status, _, err := suite.alphaArq.Write(writeBuffer, time.Now())

	suite.handleTestError(err)
	suite.Equal(windowFull, status)
}

func (suite *SelectiveArqTestSuite) TestSendingACKs() {
	segmentMtu = headerLength + 8
	suite.alphaArq.windowSize = 8
	suite.betaArq.windowSize = 8

	suite.alphaArq.Open()
	suite.betaArq.Open()

	message := "ABCDEFGHIJKLMNOPQRSTUVWXYZ123456"
	writeBuffer := []byte(message)
	readBuffer := make([]byte, segmentMtu)

	time := time.Now()

	suite.alphaArq.Write(writeBuffer, time)

	suite.betaArq.Read(readBuffer, time)
	suite.betaArq.Read(readBuffer, time)
	suite.betaArq.Read(readBuffer, time)
	suite.betaArq.Read(readBuffer, time)

	suite.alphaArq.Read(readBuffer, time)

	//bitmap should be empty and leave only the expected seqNr
	suite.Equal(uint32(2), bytesToUint32(readBuffer))

	suite.alphaArq.Read(readBuffer, time)
	suite.Equal(uint32(3), bytesToUint32(readBuffer))

	suite.alphaArq.Read(readBuffer, time)
	suite.Equal(uint32(4), bytesToUint32(readBuffer))

	suite.alphaArq.Read(readBuffer, time)
	suite.Equal(uint32(5), bytesToUint32(readBuffer))
}

func (suite *SelectiveArqTestSuite) TestRetransmission() {
	segmentMtu = headerLength + 8 //ack need 2 * uint32 space
	suite.alphaArq.windowSize = 8
	suite.betaArq.windowSize = 8
	retransmissionTimeout = 40 * time.Millisecond
	suite.alphaArq.Open()
	suite.betaArq.Open()

	suite.alphaManipulator.DropOnce(2)
	suite.alphaManipulator.DropOnce(3)

	message := "ABCD1234EFGH5678IJKL9012MNOP3456"
	writeBuffer := []byte(message)
	readBuffer := make([]byte, segmentMtu)

	now := time.Now()
	time := func() time.Time {
		now = now.Add(10 * time.Millisecond)
		return now
	}

	suite.alphaArq.Write(writeBuffer, time())

	suite.read(suite.betaArq, "ABCD1234", readBuffer, time())
	suite.read(suite.betaArq, "", readBuffer, time())

	suite.readAck(suite.alphaArq, readBuffer, time())
	suite.readAck(suite.alphaArq, readBuffer, time())

	suite.read(suite.betaArq, "", readBuffer, time())
	suite.read(suite.betaArq, "EFGH5678", readBuffer, time())

	suite.readAck(suite.alphaArq, readBuffer, time())
	suite.readAck(suite.alphaArq, readBuffer, time())
}

func TestSelectiveArq(t *testing.T) {
	suite.Run(t, new(SelectiveArqTestSuite))
}
