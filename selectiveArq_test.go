package goprotocol

import (
	"github.com/stretchr/testify/suite"
	"testing"
	"time"
)

func newMockSelectiveArqConnection(connector *ChannelConnector, name string) (*selectiveArq, *SegmentManipulator) {
	arq := selectiveArq{}
	printer := &ConsolePrinter{Name: name}
	manipulator := &SegmentManipulator{}

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
	alphaManipulator, betaManipulator *SegmentManipulator
}

func (suite *SelectiveArqTestSuite) handleTestError(err error) {
	if err != nil {
		suite.Errorf(err, "Error occurred")
	}
}
func (suite *SelectiveArqTestSuite) SetupTest() {
	endpoint1, endpoint2 := make(chan []byte, 100), make(chan []byte, 100)
	connector1, connector2 := &ChannelConnector{
		In:  endpoint1,
		Out: endpoint2,
	}, &ChannelConnector{
		In:  endpoint2,
		Out: endpoint1,
	}
	suite.alphaArq, suite.alphaManipulator = newMockSelectiveArqConnection(connector1, "alpha")
	suite.betaArq, suite.betaManipulator = newMockSelectiveArqConnection(connector2, "beta")

	suite.handleTestError(suite.alphaArq.Open())
	suite.handleTestError(suite.betaArq.Open())
}
func (suite *SelectiveArqTestSuite) TearDownTest() {
	SegmentMtu = DefaultMTU
}

func (suite *SelectiveArqTestSuite) write(c Connector, data []byte) {
	status, _, err := c.Write(data, time.Now())
	suite.handleTestError(err)
	suite.Equal(Success, status)
}
func (suite *SelectiveArqTestSuite) read(c Connector, expected string, readBuffer []byte) {
	status, n, err := c.Read(readBuffer, time.Now())
	suite.handleTestError(err)
	suite.Equal(expected, string(readBuffer[:n]))
	suite.Equal(Success, status)
}
func (suite *SelectiveArqTestSuite) readExpectStatus(c Connector, expected StatusCode, readBuffer []byte) {
	status, _, err := c.Read(readBuffer, time.Now())
	suite.handleTestError(err)
	suite.Equal(expected, status)
}
func (suite *SelectiveArqTestSuite) readAck(c Connector, readBuffer []byte) {
	suite.readExpectStatus(c, AckReceived, readBuffer)
}

func (suite *SelectiveArqTestSuite) TestQueueTimedOutSegmentsForWrite() {
	arq := selectiveArq{}
	arq.Open()

	currentTime := time.Now()

	arq.NotAckedSegment = append(arq.NotAckedSegment, &Segment{Timestamp: currentTime.Add(-1), SequenceNumber: []byte{0, 0, 0, 1}})
	arq.NotAckedSegment = append(arq.NotAckedSegment, &Segment{Timestamp: currentTime.Add(-1), SequenceNumber: []byte{0, 0, 0, 2}})
	arq.NotAckedSegment = append(arq.NotAckedSegment, &Segment{Timestamp: currentTime.Add(-1), SequenceNumber: []byte{0, 0, 0, 3}})
	arq.NotAckedSegment = append(arq.NotAckedSegment, &Segment{Timestamp: currentTime.Add(20000), SequenceNumber: []byte{0, 0, 0, 4}})
	arq.queueTimedOutSegmentsForWrite(currentTime)

	i := uint32(1)
	for !arq.readyToSendSegmentQueue.IsEmpty() {
		ele := arq.readyToSendSegmentQueue.Dequeue()
		suite.Equal(i, ele.(*Segment).GetSequenceNumber())
		suite.NotEqual(4, ele.(*Segment).GetSequenceNumber())
		i++
	}
}
func (suite *SelectiveArqTestSuite) TestWriteQueuedSegments() {

	currentTime := time.Now()

	seg1 := CreateFlaggedSegment(1, 0, []byte("test"))
	seg1.Timestamp = currentTime.Add(-1)
	seg2 := CreateFlaggedSegment(2, 0, []byte("test"))
	seg2.Timestamp = currentTime.Add(-1)
	seg3 := CreateFlaggedSegment(3, 0, []byte("test"))
	seg3.Timestamp = currentTime.Add(-1)
	seg4 := CreateFlaggedSegment(4, 0, []byte("test"))
	seg4.Timestamp = currentTime.Add(20000)

	suite.alphaArq.readyToSendSegmentQueue.Enqueue(seg1)
	suite.alphaArq.readyToSendSegmentQueue.Enqueue(seg2)
	suite.alphaArq.readyToSendSegmentQueue.Enqueue(seg3)
	suite.alphaArq.readyToSendSegmentQueue.Enqueue(seg4)

	_, _, err := suite.alphaArq.writeQueuedSegments(currentTime)

	suite.True(suite.alphaArq.readyToSendSegmentQueue.IsEmpty())
	suite.Nil(err)
}

func (suite *SelectiveArqTestSuite) TestFullWindowFlag() {
	SegmentMtu = HeaderLength + 4
	suite.alphaArq.windowSize = 8
	suite.alphaArq.Open()
	suite.betaArq.Open()

	message := "ABCDEFGHIJKLMNOPQRSTUVWXYZ123456"
	writeBuffer := []byte(message)

	status, _, err := suite.alphaArq.Write(writeBuffer, time.Now())

	suite.handleTestError(err)
	suite.Equal(WindowFull, status)
}

func (suite *SelectiveArqTestSuite) TestSendingACKs() {
	SegmentMtu = HeaderLength + 4
	suite.alphaArq.windowSize = 8
	suite.betaArq.windowSize = 8

	suite.alphaArq.Open()
	suite.betaArq.Open()

	message := "ABCDEFGHIJKLMNOPQRSTUVWXYZ123456"
	writeBuffer := []byte(message)
	readBuffer := make([]byte, SegmentMtu)

	suite.alphaArq.Write(writeBuffer, time.Now())

	suite.betaArq.Read(readBuffer, time.Now())
	suite.betaArq.Read(readBuffer, time.Now())
	suite.betaArq.Read(readBuffer, time.Now())
	suite.betaArq.Read(readBuffer, time.Now())
	suite.betaArq.Read(readBuffer, time.Now())
	suite.betaArq.Read(readBuffer, time.Now())
	suite.betaArq.Read(readBuffer, time.Now())
	suite.betaArq.Read(readBuffer, time.Now())

	suite.alphaArq.Read(readBuffer, time.Now())
	suite.Equal(uint32(2), BytesToUint32(readBuffer))

	suite.alphaArq.Read(readBuffer, time.Now())
	suite.Equal(uint32(3), BytesToUint32(readBuffer))
	suite.alphaArq.Read(readBuffer, time.Now())
	suite.Equal(uint32(4), BytesToUint32(readBuffer))
	suite.alphaArq.Read(readBuffer, time.Now())
	suite.Equal(uint32(5), BytesToUint32(readBuffer))
	suite.alphaArq.Read(readBuffer, time.Now())
	suite.Equal(uint32(6), BytesToUint32(readBuffer))
	suite.alphaArq.Read(readBuffer, time.Now())
	suite.Equal(uint32(7), BytesToUint32(readBuffer))
	suite.alphaArq.Read(readBuffer, time.Now())
	suite.Equal(uint32(8), BytesToUint32(readBuffer))
	suite.alphaArq.Read(readBuffer, time.Now())
	suite.Equal(uint32(9), BytesToUint32(readBuffer))
}

func (suite *SelectiveArqTestSuite) TestSelectiveAckDropTwoOfEight() {
	SegmentMtu = HeaderLength + 4
	suite.alphaArq.windowSize = 8
	suite.betaArq.windowSize = 8

	suite.alphaArq.Open()
	suite.betaArq.Open()

	suite.alphaManipulator.DropOnce(4)
	suite.alphaManipulator.DropOnce(5)

	message := "ABCDEFGHIJKLMNOPQRSTUVWXYZ123456"
	writeBuffer := []byte(message)
	readBuffer := make([]byte, SegmentMtu)

	suite.alphaArq.Write(writeBuffer, time.Now())

	suite.read(suite.betaArq, "ABCD", readBuffer)
	suite.read(suite.betaArq, "EFGH", readBuffer)
	suite.read(suite.betaArq, "IJKL", readBuffer)
	suite.read(suite.betaArq, "", readBuffer)
	suite.read(suite.betaArq, "", readBuffer)
	suite.read(suite.betaArq, "", readBuffer)

	suite.readAck(suite.alphaArq, readBuffer)
	suite.Equal(uint32(2), BytesToUint32(readBuffer))

	suite.readAck(suite.alphaArq, readBuffer)
	suite.Equal(uint32(3), BytesToUint32(readBuffer))

	suite.readAck(suite.alphaArq, readBuffer)
	suite.Equal(uint32(4), BytesToUint32(readBuffer))

	suite.readAck(suite.alphaArq, readBuffer)
	suite.Equal(uint32(4), BytesToUint32(readBuffer))

	suite.readAck(suite.alphaArq, readBuffer)
	suite.Equal(uint32(4), BytesToUint32(readBuffer))

	suite.readAck(suite.alphaArq, readBuffer)
	suite.Equal(uint32(4), BytesToUint32(readBuffer))
}

func (suite *SelectiveArqTestSuite) TestRetransmission() {
	SegmentMtu = HeaderLength + 8 //ack need 2 * uint32 space
	suite.alphaArq.windowSize = 8
	suite.betaArq.windowSize = 8
	RetransmissionTimeout = 40 * time.Millisecond
	suite.alphaArq.Open()
	suite.betaArq.Open()

	suite.alphaManipulator.DropOnce(2)
	suite.alphaManipulator.DropOnce(3)

	message := "ABCD1234EFGH5678IJKL9012MNOP3456"
	writeBuffer := []byte(message)
	readBuffer := make([]byte, SegmentMtu)

	now := time.Now()
	time := func() time.Time {
		now = now.Add(10 * time.Millisecond)
		return now
	}

	suite.alphaArq.Write(writeBuffer, time())
	suite.betaArq.Read(readBuffer, time())
	suite.betaArq.Read(readBuffer, time())
	suite.alphaArq.Read(readBuffer, time())
	suite.alphaArq.Read(readBuffer, time())
}

func TestSelectiveArq(t *testing.T) {
	suite.Run(t, new(SelectiveArqTestSuite))
}
