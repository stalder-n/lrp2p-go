package atp

import (
	"container/list"
	"github.com/stretchr/testify/suite"
	"time"
)

type atpTestSuite struct {
	suite.Suite
}

var testErrorChannel chan error

func init() {
	testErrorChannel = make(chan error, 100)
}

func (suite *atpTestSuite) handleTestError(err error) {
	if err != nil && len(testErrorChannel) > 0 {
		suite.Errorf(err, "Error occurred")
	}
}

func (suite *atpTestSuite) write(c connector, payload string, timestamp time.Time) int {
	return suite.writeExpectStatus(c, payload, success, timestamp)
}

func (suite *atpTestSuite) writeExpectStatus(c connector, payload string, code statusCode, timestamp time.Time) int {
	status, n, err := c.Write([]byte(payload), timestamp)
	suite.handleTestError(err)
	suite.Equal(code, status)
	return n
}

func (suite *atpTestSuite) read(c connector, expected string, timestamp time.Time) {
	readBuffer := make([]byte, segmentMtu)
	status, n, err := c.Read(readBuffer, timestamp)
	suite.handleTestError(err)
	suite.Equal(expected, string(readBuffer[:n]))
	suite.Equal(success, status)
}

func (suite *atpTestSuite) readExpectStatus(c connector, expected statusCode, timestamp time.Time) {
	readBuffer := make([]byte, segmentMtu)
	status, _, err := c.Read(readBuffer, timestamp)
	suite.handleTestError(err)
	suite.Equal(expected, status)
}

func (suite *atpTestSuite) readAck(c connector, timestamp time.Time) {
	suite.readExpectStatus(c, ackReceived, timestamp)
}

type segmentManipulator struct {
	savedSegments map[uint32][]byte
	toDropOnce    list.List
	extension     connector
}

func (manipulator *segmentManipulator) ConnectTo(remoteHost string, remotePort int) {
	manipulator.extension.ConnectTo(remoteHost, remotePort)
}

func (manipulator *segmentManipulator) Read(buffer []byte, timestamp time.Time) (statusCode, int, error) {
	return manipulator.extension.Read(buffer, timestamp)
}

func (manipulator *segmentManipulator) Close() error {
	return manipulator.extension.Close()
}

func (manipulator *segmentManipulator) AddExtension(connector connector) {
	manipulator.extension = connector
}

func (manipulator *segmentManipulator) DropOnce(sequenceNumber uint32) {
	manipulator.toDropOnce.PushFront(sequenceNumber)
}

func (manipulator *segmentManipulator) Write(buffer []byte, timestamp time.Time) (statusCode, int, error) {
	seg := createSegment(buffer)
	for elem := manipulator.toDropOnce.Front(); elem != nil; elem = elem.Next() {
		if elem.Value.(uint32) == seg.getSequenceNumber() {
			manipulator.toDropOnce.Remove(elem)
			return success, len(buffer), nil
		}
	}
	return manipulator.extension.Write(buffer, timestamp)
}

func (manipulator *segmentManipulator) SetReadTimeout(t time.Duration) {
	manipulator.extension.SetReadTimeout(t)
}

func (manipulator *segmentManipulator) reportError(err error) {
	if err != nil {
		testErrorChannel <- err
	}
}

type channelConnector struct {
	in            chan []byte
	out           chan []byte
	timeout       time.Duration
	artificialNow time.Time
}

func (connector *channelConnector) ConnectTo(remoteHost string, remotePort int) {
	panic("not implemented")
}

func (connector *channelConnector) Close() error {
	close(connector.in)
	return nil
}

func (connector *channelConnector) Write(buffer []byte, timestamp time.Time) (statusCode, int, error) {
	connector.out <- buffer
	return success, len(buffer), nil
}

func (connector *channelConnector) Read(buffer []byte, timestamp time.Time) (statusCode, int, error) {
	var buff []byte
	if connector.timeout == 0 {
		buff = <-connector.in
		copy(buffer, buff)
		return success, len(buff), nil
	}
	for {
		select {
		case buff = <-connector.in:
			if buff == nil {
				continue
			}
			copy(buffer, buff)
			return success, len(buff), nil
		case <-connector.after(timestamp, connector.timeout):
			return timeout, 0, nil
		}

	}
}

func (connector *channelConnector) SetReadTimeout(t time.Duration) {
	connector.timeout = t
	connector.in <- nil
}

func (connector *channelConnector) after(operationTime time.Time, timeout time.Duration) <-chan time.Time {
	artificialTimeout := operationTime.Sub(connector.artificialNow) + timeout
	return time.After(artificialTimeout)
}

func (connector *channelConnector) reportError(err error) {
	if err != nil {
		testErrorChannel <- err
	}
}
