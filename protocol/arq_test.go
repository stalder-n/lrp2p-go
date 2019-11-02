package protocol

import (
	"bytes"
	"container/list"
	"fmt"
	"github.com/stretchr/testify/suite"
	"reflect"
	"testing"
	"time"
)

type consolePrinter struct {
	extension Connector
	name      string
}

func (printer *consolePrinter) Open() error {
	error := printer.extension.Open()
	println(printer.name, reflect.TypeOf(printer).Elem().Name(), "Open()", "error:", fmt.Sprintf("%+v", error))
	return error
}
func (printer *consolePrinter) Close() error {
	error := printer.extension.Close()
	println(printer.name, reflect.TypeOf(printer).Elem().Name(), "Close()", "error:", fmt.Sprintf("%+v", error))
	return error;
}
func (printer *consolePrinter) addExtension(connector Connector) {
	printer.extension = connector
	println(printer.name, reflect.TypeOf(printer).Elem().Name(), "addExtension(...)", "connector:", fmt.Sprintf("%+v", connector))
}
func (printer *consolePrinter) Read(buffer []byte) (statusCode, int, error) {
	status, n, error := printer.extension.Read(buffer)
	var str string

	if bytes.Equal(buffer[FlagPosition.Start:FlagPosition.End], []byte{1}) {
		str = fmt.Sprintf("%d %d", buffer[:HeaderLength], bytes.Trim(buffer[HeaderLength:], "\x00"))
	} else if bytes.Equal(buffer[FlagPosition.Start:FlagPosition.End], []byte{0}) {
		str = fmt.Sprintf("%d %s", buffer[:HeaderLength], bytes.Trim(buffer[HeaderLength:], "\x00"))
	}
	println(printer.name, reflect.TypeOf(printer).Elem().Name(), "Read(...)", "buffer:", str, "status:", status, "n:", n, "error:", fmt.Sprintf("%+v", error))
	return status, n, error
}
func (printer *consolePrinter) Write(buffer []byte) (statusCode, int, error) {
	statusCode, n, error := printer.extension.Write(buffer)
	var str string

	if bytes.Equal(buffer[FlagPosition.Start:FlagPosition.End], []byte{1}) {
		str = fmt.Sprintf("%d %d", buffer[:HeaderLength], bytes.Trim(buffer[HeaderLength:], "\x00"))
	} else if bytes.Equal(buffer[FlagPosition.Start:FlagPosition.End], []byte{0}) {
		str = fmt.Sprintf("%d %s", buffer[:HeaderLength], bytes.Trim(buffer[HeaderLength:], "\x00"))
	}

	println(printer.name, reflect.TypeOf(printer).Elem().Name(), "Write(...)", "buffer:", str, "status:", statusCode, "n:", n, "error:", fmt.Sprintf("%+v", error))
	return statusCode, n, error
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

func newMockConnection(connector *channelConnector, name string) (*goBackNArq, *segmentManipulator) {
	arq := &goBackNArq{}
	printer := &consolePrinter{name: name}
	manipulator := &segmentManipulator{}

	arq.AddExtension(printer)
	printer.addExtension(manipulator)
	manipulator.addExtension(connector)
	return arq, manipulator
}

type GoBackNArqTestSuite struct {
	suite.Suite
	alphaArq, betaArq                 *goBackNArq
	alphaManipulator, betaManipulator *segmentManipulator
	sequenceNumberQueue               queue
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

	suite.sequenceNumberQueue = queue{}
	suite.sequenceNumberQueue.Enqueue(uint32(1))
	suite.sequenceNumberQueue.Enqueue(uint32(2))

	fu := func() uint32 {
		return suite.sequenceNumberQueue.Dequeue().(uint32)
	}

	suite.alphaArq.sequenceNumberFactory = fu
	suite.betaArq.sequenceNumberFactory = fu

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
func (suite *GoBackNArqTestSuite) TestSelectiveAckDropFourOfEight() {
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

func TestGoBackNArq(t *testing.T) {
	suite.Run(t, new(GoBackNArqTestSuite))
}
