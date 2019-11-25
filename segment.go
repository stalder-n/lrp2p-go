package goprotocol

import (
	"encoding/binary"
	"time"
)

type Position struct {
	Start int
	End   int
}

var dataOffsetPosition = Position{0, 1}
var flagPosition = Position{1, 2}

var sequenceNumberPosition = Position{2, 6}

const flagAck byte = 1
const flagSyn byte = 2
const flagEnd byte = 4

const defaultSegmentMtu = 64

var segmentMtu = defaultSegmentMtu

const headerLength = 6

var dataChunkSize = segmentMtu - headerLength

type segment struct {
	buffer         []byte
	sequenceNumber []byte
	data           []byte
	timestamp      time.Time
}

func (seg *segment) getDataOffset() byte {
	return seg.buffer[dataOffsetPosition.Start]
}

func (seg *segment) getHeaderSize() int {
	return int(seg.getDataOffset())
}

func (seg *segment) getFlags() byte {
	return seg.buffer[flagPosition.Start]
}

func (seg *segment) isFlaggedAs(flag byte) bool {
	return seg.getFlags()&flag == flag
}

func (seg *segment) getSequenceNumber() uint32 {
	return bytesToUint32(seg.sequenceNumber)
}

func (seg *segment) getExpectedSequenceNumber() uint32 {
	seqNumLength := sequenceNumberPosition.End - sequenceNumberPosition.Start;
	return bytesToUint32(seg.data[0:seqNumLength])
}

func (seg *segment) getDataAsString() string {
	return string(seg.data)
}

func setDataOffset(buffer []byte, dataOffset byte) {
	buffer[dataOffsetPosition.Start] = dataOffset
}

func setFlags(buffer []byte, flags byte) {
	buffer[flagPosition.Start] = flags
}

func setSequenceNumber(buffer []byte, sequenceNumber uint32) {
	binary.BigEndian.PutUint32(buffer[sequenceNumberPosition.Start:sequenceNumberPosition.End], sequenceNumber)
}

func createSegment(buffer []byte) segment {
	var data []byte = nil
	if len(buffer) > headerLength {
		data = buffer[buffer[dataOffsetPosition.Start]:]
	}
	return segment{
		buffer:         buffer,
		sequenceNumber: buffer[sequenceNumberPosition.Start:sequenceNumberPosition.End],
		data:           data,
	}
}

func createFlaggedSegment(sequenceNumber uint32, flags byte, data []byte) segment {
	buffer := make([]byte, headerLength+len(data))
	dataOffset := byte(headerLength)
	setDataOffset(buffer, dataOffset)
	setFlags(buffer, flags)
	setSequenceNumber(buffer, sequenceNumber)
	copy(buffer[dataOffset:], data)
	return createSegment(buffer)
}

func createAckSegment(sequenceNumber uint32) segment {
	nextSequenceNumber := make([]byte, 4)
	binary.BigEndian.PutUint32(nextSequenceNumber, sequenceNumber+1)
	return createFlaggedSegment(sequenceNumber, flagAck, nextSequenceNumber)
}

func bytesToUint32(buffer []byte) uint32 {
	return binary.BigEndian.Uint32(buffer)
}
