package goprotocol

import (
	"bytes"
	"container/list"
	"fmt"
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
	"time"
)

type ConsolePrinter struct {
	extension Connector
	Name      string
}

func (printer *ConsolePrinter) Open() error {
	err := printer.extension.Open()
	println(printer.Name, reflect.TypeOf(printer).Elem().Name(), "Open()", "error:", fmt.Sprintf("%+v", err))
	return err
}
func (printer *ConsolePrinter) Close() error {
	err := printer.extension.Close()
	println(printer.Name, reflect.TypeOf(printer).Elem().Name(), "Close()", "error:", fmt.Sprintf("%+v", err))
	return err
}
func (printer *ConsolePrinter) AddExtension(connector Connector) {
	printer.extension = connector
	println(printer.Name, reflect.TypeOf(printer).Elem().Name(), "addExtension(...)", "connector:", fmt.Sprintf("%+v", connector))
}
func (printer *ConsolePrinter) Read(buffer []byte, timestamp time.Time) (StatusCode, int, error) {
	status, n, err := printer.extension.Read(buffer, time.Now())
	printer.prettyPrint(buffer, "Read(...)", status, n, err)

	return status, n, err
}
func (printer *ConsolePrinter) Write(buffer []byte, timestamp time.Time) (StatusCode, int, error) {
	statusCode, n, err := printer.extension.Write(buffer, time.Now())
	printer.prettyPrint(buffer, "Write(...)", statusCode, n, err)

	return statusCode, n, err
}
func (printer *ConsolePrinter) prettyPrint(buffer []byte, funcName string, status StatusCode, n int, error error) {
	var str string
	if IsFlaggedAs(buffer[FlagPosition.Start], FlagACK) {
		str = fmt.Sprintf("%d %d", buffer[:HeaderLength], bytes.Trim(buffer[HeaderLength:], "\x00"))
	} else if IsFlaggedAs(buffer[FlagPosition.Start], FlagSYN) || buffer[FlagPosition.Start] == 0 {
		str = fmt.Sprintf("%d %s", buffer[:HeaderLength], bytes.Trim(buffer[HeaderLength:], "\x00"))
	} else if IsFlaggedAs(buffer[FlagPosition.Start], FlagSelectiveACK) {
		str = fmt.Sprintf("%d %d / %b", buffer[:HeaderLength], buffer[HeaderLength:], buffer[HeaderLength:])
	} else {
		str = fmt.Sprintf("CHECK_PRINTER %d %s", buffer[:HeaderLength], bytes.Trim(buffer[HeaderLength:], "\x00"))
	}
	println(printer.Name, reflect.TypeOf(printer).Elem().Name(), funcName, "buffer:", str, "status:", status, "n:", n, "error:", fmt.Sprintf("%+v", error))
}

func (printer *ConsolePrinter) SetReadTimeout(t time.Duration) {
	printer.extension.SetReadTimeout(t)
}

type SegmentManipulator struct {
	savedSegments map[uint32][]byte
	toDropOnce    list.List
	extension     Connector
}

func (manipulator *SegmentManipulator) Read(buffer []byte, timestamp time.Time) (StatusCode, int, error) {
	return manipulator.extension.Read(buffer, time.Now())
}
func (manipulator *SegmentManipulator) Open() error {
	return manipulator.extension.Open()
}
func (manipulator *SegmentManipulator) Close() error {
	return manipulator.extension.Close()
}
func (manipulator *SegmentManipulator) AddExtension(connector Connector) {
	manipulator.extension = connector
}
func (manipulator *SegmentManipulator) DropOnce(sequenceNumber uint32) {
	manipulator.toDropOnce.PushFront(sequenceNumber)
}
func (manipulator *SegmentManipulator) Write(buffer []byte, timestamp time.Time) (StatusCode, int, error) {
	seg := CreateSegment(buffer)
	for elem := manipulator.toDropOnce.Front(); elem != nil; elem = elem.Next() {
		if elem.Value.(uint32) == seg.GetSequenceNumber() {
			manipulator.toDropOnce.Remove(elem)
			return Success, len(buffer), nil
		}
	}
	return manipulator.extension.Write(buffer, time.Now())
}

func (manipulator *SegmentManipulator) SetReadTimeout(t time.Duration) {
	manipulator.extension.SetReadTimeout(t)
}

type ChannelConnector struct {
	In            chan []byte
	Out           chan []byte
	timeout       time.Duration
	artificialNow time.Time
}

func (connector *ChannelConnector) Open() error {
	return nil
}

func (connector *ChannelConnector) Close() error {
	close(connector.In)
	return nil
}

func (connector *ChannelConnector) Write(buffer []byte, timestamp time.Time) (StatusCode, int, error) {
	connector.Out <- buffer
	return Success, len(buffer), nil
}

func (connector *ChannelConnector) Read(buffer []byte, timestamp time.Time) (StatusCode, int, error) {
	var buff []byte
	if connector.timeout == 0 {
		buff = <-connector.In
		copy(buffer, buff)
		return Success, len(buff), nil
	}
	for {
		select {
		case buff = <-connector.In:
			if buff == nil {
				continue
			}
			copy(buffer, buff)
			return Success, len(buff), nil
		case <-connector.after(timestamp, connector.timeout):
			return Timeout, 0, nil
		}

	}
}

func (connector *ChannelConnector) SetReadTimeout(t time.Duration) {
	connector.timeout = t
	connector.In <- nil
}

func (connector *ChannelConnector) after(operationTime time.Time, timeout time.Duration) <-chan time.Time {
	artificialTimeout := operationTime.Sub(connector.artificialNow) + timeout
	return time.After(artificialTimeout)
}

func handleTestError(t *testing.T, err error) {
	if err != nil {
		assert.Errorf(t, err, "Error occurred")
	}
}
