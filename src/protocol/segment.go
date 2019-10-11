package protocol

import (
	"encoding/binary"
)

const segmentMtu = 64
const headerSize = 6

const indexDataOffset = 0
const indexFlags = 1

const sliceStartSeqNumber = 2
const sliceEndSeqNumber = 6

const flagAck byte = 1
const flagSyn byte = 2
const flagEnd byte = 4

type segment struct {
	buffer         []byte
	dataOffset     *byte  // Offset for data buffer
	flags          *byte  // Flags to mark specific operations
	sequenceNumber []byte // Sequence number slice
	data           []byte // Payload slice
}

func (seg *segment) flaggedAs(flag byte) bool {
	return *seg.flags&flag == flag
}

func (seg *segment) getSequenceNumber() uint32 {
	return binary.BigEndian.Uint32(seg.sequenceNumber)
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

func createDefaultSegment(sequenceNumber uint32, data []byte) segment {
	buffer := make([]byte, headerSize+len(data))
	dataOffset := byte(headerSize)
	setDataOffset(buffer, dataOffset)
	setFlags(buffer, 0)
	setSequenceNumber(buffer, sequenceNumber)
	copy(buffer[dataOffset:], data)
	return createSegment(buffer)
}

func createAckSegment(sequenceNumber uint32) segment {
	buffer := make([]byte, headerSize)
	setDataOffset(buffer, 0)
	setFlags(buffer, flagAck)
	setSequenceNumber(buffer, sequenceNumber)
	return createSegment(buffer)
}

func createSegment(buffer []byte) segment {
	var data []byte = nil
	if len(buffer) > headerSize {
		data = buffer[buffer[indexDataOffset]:]
	}
	return segment{
		buffer:         buffer,
		dataOffset:     &buffer[indexDataOffset],
		flags:          &buffer[indexFlags],
		sequenceNumber: buffer[sliceStartSeqNumber:sliceEndSeqNumber],
		data:           data,
	}
}
