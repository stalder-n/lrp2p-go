package goprotocol

import (
	"encoding/binary"
	. "go-protocol/container"
	"time"
)

var SegmentMtu = DefaultMTU

func getDataChunkSize() int {
	return SegmentMtu - HeaderLength
}

func BytesToUint32(buffer []byte) uint32 {
	return binary.BigEndian.Uint32(buffer)
}

func uint32ToBytes(data uint32) []byte {
	result := make([]byte, 4)
	binary.BigEndian.PutUint32(result, data)

	return result
}

func IsFlaggedAs(input byte, flag byte) bool {
	return input&flag == flag
}

type Segment struct {
	Buffer         []byte
	SequenceNumber []byte
	Data           []byte
	Timestamp      time.Time
}

func (seg *Segment) getDataOffset() byte {
	return seg.Buffer[DataoffsetPosition.Start]
}

func (seg *Segment) GetHeaderSize() int {
	return int(seg.getDataOffset())
}

func (seg *Segment) getFlags() byte {
	return seg.Buffer[FlagPosition.Start]
}

func (seg *Segment) IsFlaggedAs(flag byte) bool {
	return IsFlaggedAs(seg.getFlags(), flag)
}

func (seg *Segment) GetSequenceNumber() uint32 {
	return BytesToUint32(seg.SequenceNumber)
}

func (seg *Segment) getExpectedSequenceNumber() uint32 {
	seqNumLength := SequencenumberPosition.End - SequencenumberPosition.Start
	return BytesToUint32(seg.Data[0:seqNumLength])
}

func (seg *Segment) getDataAsString() string {
	return string(seg.Data)
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

func CreateSegment(buffer []byte) *Segment {
	var data []byte = nil
	if len(buffer) > HeaderLength {
		data = buffer[buffer[DataoffsetPosition.Start]:]
	}
	return &Segment{
		Buffer:         buffer,
		SequenceNumber: buffer[SequencenumberPosition.Start:SequencenumberPosition.End],
		Data:           data,
	}
}

func CreateFlaggedSegment(sequenceNumber uint32, flags byte, data []byte) *Segment {
	buffer := make([]byte, HeaderLength+len(data))
	dataOffset := byte(HeaderLength)
	setDataOffset(buffer, dataOffset)
	setFlags(buffer, flags)
	setSequenceNumber(buffer, sequenceNumber)
	copy(buffer[dataOffset:], data)
	return CreateSegment(buffer)
}

func CreateAckSegment(sequenceNumber uint32, receivedSequenceNumber uint32) *Segment {
	receivedSequenceNumberBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(receivedSequenceNumberBytes, receivedSequenceNumber)
	return CreateFlaggedSegment(sequenceNumber, FlagACK, receivedSequenceNumberBytes)
}

func CreateSelectiveAckSegment(sequenceNumber uint32, bitmap *Bitmap) *Segment {
	first := uint32ToBytes(bitmap.SeqNumber)
	second := uint32ToBytes(bitmap.ToNumber())

	return CreateFlaggedSegment(sequenceNumber, FlagSelectiveACK, append(first, second...))
}

func CreateSegments(buffer []byte, seqNumFactory func() uint32) *Queue {
	result := NewQueue()

	var seg *Segment
	currentIndex := 0
	for {
		currentIndex, seg = PeekFlaggedSegmentOfBuffer(currentIndex, seqNumFactory(), buffer)
		result.Enqueue(seg)
		if currentIndex == len(buffer) {
			break
		}
	}

	return result
}

func PeekFlaggedSegmentOfBuffer(currentIndex int, sequenceNum uint32, buffer []byte) (int, *Segment) {
	var next = currentIndex + getDataChunkSize()
	var flag byte = 0
	if currentIndex == 0 {
		flag |= FlagSYN
	}
	if next >= len(buffer) {
		next = len(buffer)
	}
	return next, CreateFlaggedSegment(sequenceNum, flag, buffer[currentIndex:next])
}
