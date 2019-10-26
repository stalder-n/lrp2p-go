package protocol

import (
	"encoding/binary"
	"time"
)

type Position struct {
	Start int
	End   int
}

const headerLength = 6

var DataOffsetPosition = Position{0, 1}
var FlagPosition = Position{1, 2}

var SequenceNumberPosition = Position{2, 6}

const flagAck byte = 1
const flagSyn byte = 2
const flagEnd byte = 4

const defaultSegmentMtu = 64

var segmentMtu = defaultSegmentMtu
var dataChunkSize = segmentMtu - headerLength

type segment struct {
	buffer         []byte
	sequenceNumber []byte
	data           []byte
	timestamp      time.Time
}

func (seg *segment) getDataOffset() byte {
	return seg.buffer[DataOffsetPosition.Start]
}

func (seg *segment) getHeaderSize() int {
	return int(seg.getDataOffset())
}

func (seg *segment) getFlags() byte {
	return seg.buffer[FlagPosition.Start]
}

func (seg *segment) isFlaggedAs(flag byte) bool {
	return seg.getFlags()&flag == flag
}

func (seg *segment) getSequenceNumber() uint32 {
	return bytesToUint32(seg.sequenceNumber)
}

func (seg *segment) getExpectedSequenceNumber() uint32 {
	return bytesToUint32(seg.data[SequenceNumberPosition.Start:SequenceNumberPosition.End])
}

func (seg *segment) getDataAsString() string {
	return string(seg.data)
}

func setDataOffset(buffer []byte, dataOffset byte) {
	buffer[DataOffsetPosition.Start] = dataOffset
}

func setFlags(buffer []byte, flags byte) {
	buffer[FlagPosition.Start] = flags
}

func setSequenceNumber(buffer []byte, sequenceNumber uint32) {
	binary.BigEndian.PutUint32(buffer[SequenceNumberPosition.Start:SequenceNumberPosition.End], sequenceNumber)
}

func createSegment(buffer []byte) segment {
	var data []byte = nil
	if len(buffer) > headerLength {
		data = buffer[buffer[DataOffsetPosition.Start]:]
	}
	return segment{
		buffer:         buffer,
		sequenceNumber: buffer[SequenceNumberPosition.Start:SequenceNumberPosition.End],
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
