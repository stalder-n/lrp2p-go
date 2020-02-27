package atp

import (
	"bytes"
	"container/list"
	"flag"
	"fmt"
	"github.com/stretchr/testify/suite"
	"reflect"
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

var flagVerbose = flag.Bool("v", false, "show more detailed console output")

type consolePrinter struct {
	extension connector
	Name      string
}

func (printer *consolePrinter) Close() error {
	err := printer.extension.Close()
	if *flagVerbose {
		println(printer.Name, reflect.TypeOf(printer).Elem().Name(), "Close()", "error:", fmt.Sprintf("%+v", err))
	}
	return err
}

func (printer *consolePrinter) AddExtension(connector connector) {
	printer.extension = connector
	if *flagVerbose {
		println(printer.Name, reflect.TypeOf(printer).Elem().Name(), "addExtension(...)", "connector:", fmt.Sprintf("%+v", connector))
	}
}

func (printer *consolePrinter) Read(buffer []byte, timestamp time.Time) (statusCode, int, error) {
	status, n, err := printer.extension.Read(buffer, time.Now())
	if *flagVerbose {
		printer.prettyPrint(buffer, "Read(...)", status, n, err)
	}

	return status, n, err
}

func (printer *consolePrinter) Write(buffer []byte, timestamp time.Time) (statusCode, int, error) {
	statusCode, n, err := printer.extension.Write(buffer, time.Now())
	if *flagVerbose {
		printer.prettyPrint(buffer, "Write(...)", statusCode, n, err)
	}

	return statusCode, n, err
}

func (printer *consolePrinter) prettyPrint(buffer []byte, funcName string, status statusCode, n int, error error) {
	var str string
	if isFlaggedAs(buffer[flagPosition.Start], flagSYN) || buffer[flagPosition.Start] == 0 {
		str = fmt.Sprintf("%d %s", buffer[:headerLength], bytes.Trim(buffer[headerLength:], "\x00"))
	} else if isFlaggedAs(buffer[flagPosition.Start], flagACK) {
		str = fmt.Sprintf("%d %d / %b", buffer[:headerLength], buffer[headerLength:], buffer[headerLength:])
	} else {
		str = fmt.Sprintf("CHECK_PRINTER %d %s", buffer[:headerLength], bytes.Trim(buffer[headerLength:], "\x00"))
	}
	println(printer.Name, reflect.TypeOf(printer).Elem().Name(), funcName, "buffer:", str, "status:", status, "n:", n, "error:", fmt.Sprintf("%+v", error))
}

func (printer *consolePrinter) SetReadTimeout(t time.Duration) {
	printer.extension.SetReadTimeout(t)
}

func (printer *consolePrinter) reportError(err error) {
	if err != nil {
		testErrorChannel <- err
	}
}

type segmentManipulator struct {
	savedSegments map[uint32][]byte
	toDropOnce    list.List
	extension     connector
}

func (manipulator *segmentManipulator) Read(buffer []byte, timestamp time.Time) (statusCode, int, error) {
	return manipulator.extension.Read(buffer, time.Now())
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
	return manipulator.extension.Write(buffer, time.Now())
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
