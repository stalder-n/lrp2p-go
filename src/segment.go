package main

import (
	"encoding/binary"
)

const SegmentMtu = 64
const HeaderSize = 6

const IndexDataOffset = 0
const IndexFlags = 1

const SliceStartSeqNumber = 2
const SliceEndSeqNumber = 6

const FlagAck = 1
const FlagEnd = 2

type segment struct {
	buffer         []byte
	dataOffset     *byte  // Offset for data buffer
	flags          *byte  // Flags to mark specific operations
	sequenceNumber []byte // Sequence number slice
	data           []byte // Payload slice
}

func (seg segment) getSequenceNumber() uint32 {
	return binary.BigEndian.Uint32(seg.sequenceNumber)
}

func (seg segment) getDataAsString() string {
	return string(seg.data)
}

func setDataOffset(buffer []byte, dataOffset byte) {
	buffer[IndexDataOffset] = dataOffset
}

func setFlags(buffer []byte, flags byte) {
	buffer[IndexFlags] = flags
}

func setSequenceNumber(buffer []byte, sequenceNumber uint32) {
	binary.BigEndian.PutUint32(buffer[SliceStartSeqNumber:SliceEndSeqNumber], sequenceNumber)
}

func createDefaultSegment(sequenceNumber uint32, data []byte) segment {
	buffer := make([]byte, HeaderSize+len(data))
	dataOffset := byte(HeaderSize)
	setDataOffset(buffer, dataOffset)
	setFlags(buffer, 0)
	setSequenceNumber(buffer, sequenceNumber)
	copy(buffer[dataOffset:], data)
	return createSegment(buffer)
}

func createAckSegment(sequenceNumber uint32) segment {
	buffer := make([]byte, HeaderSize)
	setDataOffset(buffer, 0)
	setFlags(buffer, FlagAck)
	setSequenceNumber(buffer, sequenceNumber)
	return createSegment(buffer)
}

func createSegment(buffer []byte) segment {
	var data []byte = nil
	if len(buffer) > HeaderSize {
		data = buffer[buffer[IndexDataOffset]:]
	}
	return segment{
		buffer:         buffer,
		dataOffset:     &buffer[IndexDataOffset],
		flags:          &buffer[IndexFlags],
		sequenceNumber: buffer[SliceStartSeqNumber:SliceEndSeqNumber],
		data:           data,
	}
}
