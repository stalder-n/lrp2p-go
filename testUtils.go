package lrp2p

import (
	"github.com/stretchr/testify/suite"
	"strings"
	"time"
)

type lrp2pTestSuite struct {
	suite.Suite
	timestamp time.Time
}

const localhost = "127.0.0.1"

var testErrorChannel chan error

func init() {
	testErrorChannel = make(chan error, 100)
}

func (suite *lrp2pTestSuite) handleTestError(err error) {
	if err != nil && len(testErrorChannel) > 0 {
		suite.Errorf(err, "Error occurred")
	}
}

type segmentManipulator struct {
	savedSegments map[uint32][]byte
	toDropOnce    []uint32
	conn          connector
}

type channelConnector struct {
	in            chan []byte
	out           chan []byte
	timeout       time.Duration
	artificialNow time.Time
}

func (manipulator *segmentManipulator) Read(buffer []byte, timestamp time.Time) (statusCode, int, error) {
	return manipulator.conn.Read(buffer, timestamp)
}

func (manipulator *segmentManipulator) Write(buffer []byte) (statusCode, int, error) {
	seg := createSegment(buffer)
	for i := 0; i < len(manipulator.toDropOnce); i++ {
		if manipulator.toDropOnce[i] == seg.getSequenceNumber() {
			manipulator.toDropOnce = append(manipulator.toDropOnce[:i], manipulator.toDropOnce[i+1:]...)
			i--
			return success, len(buffer), nil
		}
	}
	return manipulator.conn.Write(buffer)
}

func (manipulator *segmentManipulator) DropOnce(sequenceNumber uint32) {
	manipulator.toDropOnce = append(manipulator.toDropOnce, sequenceNumber)
}

func (manipulator *segmentManipulator) SetReadTimeout(t time.Duration) {
	manipulator.conn.SetReadTimeout(t)
}

func (manipulator *segmentManipulator) Close() error {
	return manipulator.conn.Close()
}

func (manipulator *segmentManipulator) reportError(err error) {
	if err != nil {
		testErrorChannel <- err
	}
}

type connectionManipulator struct {
	conn       connector
	writeDelay time.Duration
}

func (manipulator *connectionManipulator) Read(buffer []byte, timestamp time.Time) (statusCode, int, error) {
	return manipulator.conn.Read(buffer, timestamp)
}

func (manipulator *connectionManipulator) Close() error {
	return manipulator.conn.Close()
}

func (manipulator *connectionManipulator) Write(buffer []byte) (statusCode, int, error) {
	time.Sleep(manipulator.writeDelay)
	return manipulator.conn.Write(buffer)
}

func (manipulator *connectionManipulator) SetReadTimeout(t time.Duration) {
	manipulator.conn.SetReadTimeout(t)
}

func (manipulator *connectionManipulator) reportError(err error) {
	if err != nil {
		testErrorChannel <- err
	}
}

func (connector *channelConnector) Close() error {
	close(connector.in)
	return nil
}

func (connector *channelConnector) Write(buffer []byte) (statusCode, int, error) {
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
}

func (connector *channelConnector) after(operationTime time.Time, timeout time.Duration) <-chan time.Time {
	artificialTimeout := timeout - operationTime.Sub(connector.artificialNow)
	return time.After(artificialTimeout)
}

func (connector *channelConnector) reportError(err error) {
	if err != nil {
		testErrorChannel <- err
	}
}

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
