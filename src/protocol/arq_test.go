package protocol

import (
	"container/list"
	"os"
	"testing"
)

var connection1, connection2 *connection
var arq1, arq2 *goBackNArq
var manipulator1, manipulator2 *segmentManipulator
var initialSequenceNumberQueue queue

func handleTestError(err error, t *testing.T) {
	if err != nil {
		t.Error("Error occurred:", err.Error())
	}
}

func TestMain(m *testing.M) {
	setup()
	code := m.Run()
	shutdown()
	os.Exit(code)
}

func setup() {
	mockConnections()
	initialSequenceNumberQueue = queue{}
	sequenceNumberFactory = func() uint32 {
		return initialSequenceNumberQueue.Dequeue().(uint32)
	}
}

func shutdown() {
	connection1.Close()
	connection2.Close()
}

func mockConnections() {
	endpoint1, endpoint2 := make(chan []byte, 100), make(chan []byte, 100)
	connector1, connector2 := &channelConnector{
		in:  endpoint1,
		out: endpoint2,
	}, &channelConnector{
		in:  endpoint2,
		out: endpoint1,
	}
	connection1, arq1, manipulator1 = newMockConnection(connector1)
	connection2, arq2, manipulator2 = newMockConnection(connector2)
	connection1.Open()
	connection2.Open()
}

func newMockConnection(connector *channelConnector) (*connection, *goBackNArq, *segmentManipulator) {
	connection := &connection{}
	arq := &goBackNArq{}
	manipulator := &segmentManipulator{}
	adapter := &connectorAdapter{connector}
	connection.addExtension(arq)
	arq.addExtension(manipulator)
	manipulator.addExtension(adapter)
	return connection, arq, manipulator
}

type segmentManipulator struct {
	extensionDelegator
	savedSegments map[uint32][]byte
	toDropOnce    list.List
}

func (manipulator *segmentManipulator) dropOnce(sequenceNumber uint32) {
	manipulator.toDropOnce.PushFront(sequenceNumber)
}

func (manipulator *segmentManipulator) Write(buffer []byte) (int, error) {
	seg := createSegment(buffer)
	for elem := manipulator.toDropOnce.Front(); elem != nil; elem = elem.Next() {
		if elem.Value.(uint32) == seg.getSequenceNumber() {
			manipulator.toDropOnce.Remove(elem)
			return len(buffer), nil
		}
	}
	return manipulator.extension.Write(buffer)
}

type channelConnector struct {
	in  chan []byte
	out chan []byte
}

func (connector *channelConnector) Open() {
}

func (connector *channelConnector) Close() {
	close(connector.in)
}

func (connector *channelConnector) Write(buffer []byte) (int, error) {
	connector.out <- buffer
	return len(buffer), nil
}

func (connector *channelConnector) Read(buffer []byte) (int, error) {
	copy(buffer, <-connector.in)
	return len(buffer), nil

}
