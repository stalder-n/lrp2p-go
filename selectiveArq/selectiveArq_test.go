package selectiveArq

import (
	"github.com/stretchr/testify/suite"
	. "go-protocol"
	. "go-protocol/container"
	. "go-protocol/lowlevel"
	"testing"
	"time"
)

func newMockSelectiveArqConnection(connector *ChannelConnector, name string) (*selectiveArq, *SegmentManipulator) {
	arq := selectiveArq{}
	printer := &ConsolePrinter{Name: name}
	manipulator := &SegmentManipulator{}

	arq.addExtension(printer)
	printer.AddExtension(manipulator)
	manipulator.AddExtension(connector)

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
	sequenceNumberQueue               Queue
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

	suite.sequenceNumberQueue = Queue{}
	suite.sequenceNumberQueue.New()
	suite.sequenceNumberQueue.Enqueue(uint32(1))
	suite.sequenceNumberQueue.Enqueue(uint32(2))

	suite.handleTestError(suite.alphaArq.Open())
	suite.handleTestError(suite.betaArq.Open())
}
func (suite *SelectiveArqTestSuite) TearDownTest() {
	SegmentMtu = DefaultMTU
}

func (suite *SelectiveArqTestSuite) write(c Connector, data []byte) {
	status, _, err := c.Write(data)
	suite.handleTestError(err)
	suite.Equal(Success, status)
}
func (suite *SelectiveArqTestSuite) read(c Connector, expected string, readBuffer []byte) {
	status, n, err := c.Read(readBuffer)
	suite.handleTestError(err)
	suite.Equal(expected, string(readBuffer[:n]))
	suite.Equal(Success, status)
}
func (suite *SelectiveArqTestSuite) readExpectStatus(c Connector, expected StatusCode, readBuffer []byte) {
	status, _, err := c.Read(readBuffer)
	suite.handleTestError(err)
	suite.Equal(expected, status)
}
func (suite *SelectiveArqTestSuite) readAck(c Connector, readBuffer []byte) {
	suite.readExpectStatus(c, AckReceived, readBuffer)
}

func (suite *SelectiveArqTestSuite) TestQueueTimedOutSegmentsForWrite() {
	arq := selectiveArq{}

	time := time.Now()

	arq.notAckedSegment = append(arq.notAckedSegment, &Segment{Timestamp: time.Add(-1), SequenceNumber: []byte{0, 0, 0, 1}})
	arq.notAckedSegment = append(arq.notAckedSegment, &Segment{Timestamp: time.Add(-1), SequenceNumber: []byte{0, 0, 0, 2}})
	arq.notAckedSegment = append(arq.notAckedSegment, &Segment{Timestamp: time.Add(-1), SequenceNumber: []byte{0, 0, 0, 3}})
	arq.notAckedSegment = append(arq.notAckedSegment, &Segment{Timestamp: time.Add(20000), SequenceNumber: []byte{0, 0, 0, 4}})
	arq.queueTimedOutSegmentsForWrite()

	i := uint32(1)
	for !arq.readyToSendSegmentQueue.IsEmpty() {
		ele := arq.readyToSendSegmentQueue.Dequeue()
		suite.Equal(i, ele.(*Segment).GetSequenceNumber())
		suite.NotEqual(4, ele.(*Segment).GetSequenceNumber())
		i++
	}
}
func (suite *SelectiveArqTestSuite) TestWriteQueuedSegments() {

	time := time.Now()

	suite.alphaArq.readyToSendSegmentQueue.Enqueue(&Segment{Timestamp: time.Add(-1), SequenceNumber: []byte{0, 0, 0, 1}})
	suite.alphaArq.readyToSendSegmentQueue.Enqueue(&Segment{Timestamp: time.Add(-1), SequenceNumber: []byte{0, 0, 0, 2}})
	suite.alphaArq.readyToSendSegmentQueue.Enqueue(&Segment{Timestamp: time.Add(-1), SequenceNumber: []byte{0, 0, 0, 3}})
	suite.alphaArq.readyToSendSegmentQueue.Enqueue(&Segment{Timestamp: time.Add(20000), SequenceNumber: []byte{0, 0, 0, 4}})

	_, _, error := suite.alphaArq.writeQueuedSegments()

	suite.True(suite.alphaArq.readyToSendSegmentQueue.IsEmpty())
	suite.Nil(error)

}

func (suite *SelectiveArqTestSuite) TestSelectiveAckDropFourOfEight() {
	SegmentMtu = HeaderLength + 4

	suite.alphaManipulator.DropOnce(5)
	suite.alphaManipulator.DropOnce(6)
	suite.alphaManipulator.DropOnce(7)

	RetransmissionTimeout = 20 * time.Millisecond
	message := "ABCDEFGHIJKLMNOPQRSTUVWXYZ123456"
	writeBuffer := []byte(message)
	readBuffer := make([]byte, SegmentMtu)
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
func (suite *SelectiveArqTestSuite) disabled_TestSendInOneSegmentSelectiveACK() {
	message := "Hello, World!"
	writeBuffer := []byte(message)
	readBuffer := make([]byte, SegmentMtu)

	alpha := selectiveArq{}
	alpha.addExtension(suite.alphaArq.extension)
	beta := selectiveArq{}
	beta.addExtension(suite.betaArq.extension)
	suite.alphaArq = &alpha
	suite.betaArq = &beta
	suite.write(suite.alphaArq, writeBuffer)
	suite.read(suite.betaArq, message, readBuffer)
	suite.readAck(suite.alphaArq, readBuffer)
}

func TestSelectiveArq(t *testing.T) {
	suite.Run(t, new(SelectiveArqTestSuite))
}
