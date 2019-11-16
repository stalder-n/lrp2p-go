package go_protocol

import (
	"bytes"
	"container/list"
	"fmt"
	. "go-protocol/lowlevel"
	"reflect"
)

type ConsolePrinter struct {
	extension Connector
	Name      string
}

func (printer *ConsolePrinter) Open() error {
	error := printer.extension.Open()
	println(printer.Name, reflect.TypeOf(printer).Elem().Name(), "Open()", "error:", fmt.Sprintf("%+v", error))
	return error
}
func (printer *ConsolePrinter) Close() error {
	error := printer.extension.Close()
	println(printer.Name, reflect.TypeOf(printer).Elem().Name(), "Close()", "error:", fmt.Sprintf("%+v", error))
	return error
}
func (printer *ConsolePrinter) AddExtension(connector Connector) {
	printer.extension = connector
	println(printer.Name, reflect.TypeOf(printer).Elem().Name(), "addExtension(...)", "connector:", fmt.Sprintf("%+v", connector))
}
func (printer *ConsolePrinter) Read(buffer []byte) (StatusCode, int, error) {
	status, n, error := printer.extension.Read(buffer)
	printer.prettyPrint(buffer, "Read(...)", status, n, error)

	return status, n, error
}
func (printer *ConsolePrinter) Write(buffer []byte) (StatusCode, int, error) {
	statusCode, n, error := printer.extension.Write(buffer)
	printer.prettyPrint(buffer, "Write(...)", statusCode, n, error)

	return statusCode, n, error
}
func (printer *ConsolePrinter) prettyPrint(buffer []byte, funcName string, status StatusCode, n int, error error) {
	var str string
	if IsFlaggedAs(buffer[FlagPosition.Start], FlagACK) {
		str = fmt.Sprintf("%d %d", buffer[:HeaderLength], bytes.Trim(buffer[HeaderLength:], "\x00"))
	} else if IsFlaggedAs(buffer[FlagPosition.Start], FlagSYN) || buffer[FlagPosition.Start] == 0 {
		str = fmt.Sprintf("%d %s", buffer[:HeaderLength], bytes.Trim(buffer[HeaderLength:], "\x00"))
	} else if IsFlaggedAs(buffer[FlagPosition.Start], FlagSelectiveACK) {
		str = fmt.Sprintf("%d %b / %d", buffer[:HeaderLength], bytes.Trim(buffer[HeaderLength:], "\x00"), bytes.Trim(buffer[HeaderLength:], "\x00"))
	} else {
		str = fmt.Sprintf("CHECK_PRINTER %d %s", buffer[:HeaderLength], bytes.Trim(buffer[HeaderLength:], "\x00"))
	}
	println(printer.Name, reflect.TypeOf(printer).Elem().Name(), funcName, "buffer:", str, "status:", status, "n:", n, "error:", fmt.Sprintf("%+v", error))
}

type SegmentManipulator struct {
	savedSegments map[uint32][]byte
	toDropOnce    list.List
	extension     Connector
}

func (manipulator *SegmentManipulator) Read(buffer []byte) (StatusCode, int, error) {
	return manipulator.extension.Read(buffer)
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
func (manipulator *SegmentManipulator) Write(buffer []byte) (StatusCode, int, error) {
	seg := CreateSegment(buffer)
	for elem := manipulator.toDropOnce.Front(); elem != nil; elem = elem.Next() {
		if elem.Value.(uint32) == seg.GetSequenceNumber() {
			manipulator.toDropOnce.Remove(elem)
			return Success, len(buffer), nil
		}
	}
	return manipulator.extension.Write(buffer)
}

type ChannelConnector struct {
	In  chan []byte
	Out chan []byte
}

func (connector *ChannelConnector) Open() error {
	return nil
}
func (connector *ChannelConnector) Close() error {
	close(connector.In)
	return nil
}
func (connector *ChannelConnector) Write(buffer []byte) (StatusCode, int, error) {
	connector.Out <- buffer
	return Success, len(buffer), nil
}
func (connector *ChannelConnector) Read(buffer []byte) (StatusCode, int, error) {
	buff := <-connector.In
	copy(buffer, buff)
	return Success, len(buff), nil
}
