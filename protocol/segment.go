package protocol

import (
	"encoding/binary"
	"time"
)

var segmentMtu = DefaultMTU

type segment struct {
	buffer         []byte
	sequenceNumber []byte
	data           []byte
	timestamp      time.Time
}

func (seg *segment) getDataOffset() byte {
	return seg.buffer[DataoffsetPosition.Start]
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
	seqNumLength := SequencenumberPosition.End- SequencenumberPosition.Start;
	return bytesToUint32(seg.data[0:seqNumLength])
}

func (seg *segment) getDataAsString() string {
	return string(seg.data)
}

func setDataOffset(buffer []byte, dataOffset byte) {
	buffer[DataoffsetPosition.Start] = dataOffset
}

func setFlags(buffer []byte, flags byte) {
	buffer[FlagPosition.Start] = flags
}

func setSequenceNumber(buffer []byte, sequenceNumber uint32) {
	binary.BigEndian.PutUint32(buffer[SequencenumberPosition.Start:SequencenumberPosition.End], sequenceNumber)
}

func createSegment(buffer []byte) segment {
	var data []byte = nil
	if len(buffer) > HeaderLength {
		data = buffer[buffer[DataoffsetPosition.Start]:]
	}
	return segment{
		buffer:         buffer,
		sequenceNumber: buffer[SequencenumberPosition.Start:SequencenumberPosition.End],
		data:           data,
	}
}

func peekFlaggedSegmentOfBuffer(currentIndex int, sequenceNum uint32, buffer []byte) (int, segment) {
	var next = currentIndex + getDataChunkSize();
	var flag byte = 0
	if currentIndex == 0 {
		flag |= FlagSYN
	}
	if next >= len(buffer) {
		flag |= FlagEND
		next = len(buffer)
	}
	return next, createFlaggedSegment(sequenceNum, flag, buffer[currentIndex:next])
}

func createFlaggedSegment(sequenceNumber uint32, flags byte, data []byte) segment {
	buffer := make([]byte, HeaderLength+len(data))
	dataOffset := byte(HeaderLength)
	setDataOffset(buffer, dataOffset)
	setFlags(buffer, flags)
	setSequenceNumber(buffer, sequenceNumber)
	copy(buffer[dataOffset:], data)
	return createSegment(buffer)
}

func createAckSegment(sequenceNumber uint32, receivedSequenceNumber uint32) segment {
	receivedSequenceNumberBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(receivedSequenceNumberBytes, receivedSequenceNumber)
	return createFlaggedSegment(sequenceNumber, FlagACK, receivedSequenceNumberBytes)
}

func getDataChunkSize() int {
	return segmentMtu - HeaderLength;
}

func bytesToUint32(buffer []byte) uint32 {
	return binary.BigEndian.Uint32(buffer)
}
