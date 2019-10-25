package protocol

import (
	"encoding/binary"
	"time"
)

const headerSize = 6

const indexDataOffset = 0
const indexFlags = 1

const sliceStartSeqNumber = 2
const sliceEndSeqNumber = 6
const seqNumberLength = sliceEndSeqNumber - sliceStartSeqNumber

const flagAck byte = 1
const flagSyn byte = 2
const flagEnd byte = 4

const defaultSegmentMtu = 64

var segmentMtu = defaultSegmentMtu
var dataChunkSize = segmentMtu - headerSize

type segment struct {
	buffer         []byte
	sequenceNumber []byte
	data           []byte
	timestamp      time.Time
}

func (seg *segment) dataOffset() byte {
	return seg.buffer[indexDataOffset]
}

func (seg *segment) headerSize() int {
	return int(seg.dataOffset())
}

func (seg *segment) flags() byte {
	return seg.buffer[indexFlags]
}

func (seg *segment) flaggedAs(flag byte) bool {
	return seg.flags()&flag == flag
}

func (seg *segment) getSequenceNumber() uint32 {
	return bytesToUint32(seg.sequenceNumber)
}

func (seg *segment) getExpectedSequenceNumber() uint32 {
	return bytesToUint32(seg.data[:seqNumberLength])
}

func (seg *segment) getDataAsString() string {
	return string(seg.data)
}

func setDataOffset(buffer []byte, dataOffset byte) {
	buffer[indexDataOffset] = dataOffset
}

func setFlags(buffer []byte, flags byte) {
	buffer[indexFlags] = flags
}

func setSequenceNumber(buffer []byte, sequenceNumber uint32) {
	binary.BigEndian.PutUint32(buffer[sliceStartSeqNumber:sliceEndSeqNumber], sequenceNumber)
}

func createFlaggedSegment(sequenceNumber uint32, flags byte, data []byte) segment {
	buffer := make([]byte, headerSize+len(data))
	dataOffset := byte(headerSize)
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

func createSegment(buffer []byte) segment {
	var data []byte = nil
	if len(buffer) > headerSize {
		data = buffer[buffer[indexDataOffset]:]
	}
	return segment{
		buffer:         buffer,
		sequenceNumber: buffer[sliceStartSeqNumber:sliceEndSeqNumber],
		data:           data,
	}
}

func bytesToUint32(buffer []byte) uint32 {
	return binary.BigEndian.Uint32(buffer)
}
