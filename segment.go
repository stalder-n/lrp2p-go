package atp

import (
	"encoding/binary"
	"time"
)

var segmentMtu = defaultMTU

func getDataChunkSize() int {
	return segmentMtu - headerLength
}

func bytesToUint32(buffer []byte) uint32 {
	return binary.BigEndian.Uint32(buffer)
}

func uint32ToBytes(data uint32) []byte {
	result := make([]byte, 4)
	binary.BigEndian.PutUint32(result, data)
	return result
}

func isFlaggedAs(input byte, flag byte) bool {
	return input&flag == flag
}

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
	return isFlaggedAs(seg.getFlags(), flag)
}

func (seg *segment) getSequenceNumber() uint32 {
	return bytesToUint32(seg.sequenceNumber)
}

func (seg *segment) getExpectedSequenceNumber() uint32 {
	seqNumLength := sequenceNumberPosition.End - sequenceNumberPosition.Start
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

func createSegment(buffer []byte) *segment {
	var data []byte = nil
	if len(buffer) > headerLength {
		data = buffer[buffer[dataOffsetPosition.Start]:]
	}
	return &segment{
		buffer:         buffer,
		sequenceNumber: buffer[sequenceNumberPosition.Start:sequenceNumberPosition.End],
		data:           data,
	}
}

func createFlaggedSegment(sequenceNumber uint32, flags byte, data []byte) *segment {
	buffer := make([]byte, headerLength+len(data))
	dataOffset := byte(headerLength)
	setDataOffset(buffer, dataOffset)
	setFlags(buffer, flags)
	setSequenceNumber(buffer, sequenceNumber)
	copy(buffer[dataOffset:], data)
	return createSegment(buffer)
}

func createAckSegment(sequenceNumber uint32, receivedSequenceNumber uint32) *segment {
	receivedSequenceNumberBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(receivedSequenceNumberBytes, receivedSequenceNumber)
	return createFlaggedSegment(sequenceNumber, flagACK, receivedSequenceNumberBytes)
}

func createSelectiveAckSegment(sequenceNumber uint32, bitmap *bitmap) *segment {
	first := uint32ToBytes(bitmap.sequenceNumber)
	second := uint32ToBytes(bitmap.ToNumber())

	data := append(first, second...)

	return createFlaggedSegment(sequenceNumber, flagACK, data)
}

func createSegments(buffer []byte, seqNumFactory func() uint32) *queue {
	result := newQueue()

	var seg *segment
	currentIndex := 0
	for {
		currentIndex, seg = peekFlaggedSegmentOfBuffer(currentIndex, seqNumFactory(), buffer)
		result.Enqueue(seg)
		if currentIndex == len(buffer) {
			break
		}
	}

	return result
}

func peekFlaggedSegmentOfBuffer(currentIndex int, sequenceNum uint32, buffer []byte) (int, *segment) {
	var next = currentIndex + getDataChunkSize()
	var flag byte = 0
	if currentIndex == 0 {
		flag |= flagSYN
	}
	if next >= len(buffer) {
		next = len(buffer)
	}
	return next, createFlaggedSegment(sequenceNum, flag, buffer[currentIndex:next])
}
