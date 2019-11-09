package protocol

import (
	"bytes"
	"container/list"
	"fmt"
	"reflect"
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

	if isFlaggedAs(buffer[FlagPosition.Start], FlagACK) {
		str = fmt.Sprintf("%d %d", buffer[:HeaderLength], bytes.Trim(buffer[HeaderLength:], "\x00"))
	} else if isFlaggedAs(buffer[FlagPosition.Start], FlagSYN) || isFlaggedAs(buffer[FlagPosition.Start], FlagSelectiveACK) || buffer[FlagPosition.Start] == 0 {
		str = fmt.Sprintf("%d %s", buffer[:HeaderLength], bytes.Trim(buffer[HeaderLength:], "\x00"))
	} else {
		str = fmt.Sprintf("CHECK_PRINTER %d %s", buffer[:HeaderLength], bytes.Trim(buffer[HeaderLength:], "\x00"))
	}

	println(printer.name, reflect.TypeOf(printer).Elem().Name(), "Read(...)", "buffer:", str, "status:", status, "n:", n, "error:", fmt.Sprintf("%+v", error))
	return status, n, error
}
func (printer *consolePrinter) Write(buffer []byte) (statusCode, int, error) {
	statusCode, n, error := printer.extension.Write(buffer)
	var str string

	if isFlaggedAs(buffer[FlagPosition.Start], FlagACK) {
		str = fmt.Sprintf("%d %d", buffer[:HeaderLength], bytes.Trim(buffer[HeaderLength:], "\x00"))
	} else if isFlaggedAs(buffer[FlagPosition.Start], FlagSYN) || isFlaggedAs(buffer[FlagPosition.Start], FlagSelectiveACK) || buffer[FlagPosition.Start] == 0 {
		str = fmt.Sprintf("%d %s", buffer[:HeaderLength], bytes.Trim(buffer[HeaderLength:], "\x00"))
	} else {
		str = fmt.Sprintf("CHECK_PRINTER: %d %s", buffer[:HeaderLength], bytes.Trim(buffer[HeaderLength:], "\x00"))
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
