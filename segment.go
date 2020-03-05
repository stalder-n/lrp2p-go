package atp

import (
	"encoding/binary"
	"time"
)

var segmentMtu = defaultMTU

const (
	ackDelimSeq   byte = 1
	ackDelimRange byte = 2
	ackDelimEnd   byte = 4
)

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

func isInSequence(seg1, seg2 *segment) bool {
	return seg1.getSequenceNumber()+1 == seg2.getSequenceNumber()
}

func createAckSegment(sequenceNumber uint32, segmentBuffer []*segment) *segment {
	data := make([]byte, 0, segmentMtu)
	var prevSeg *segment
	var lastDelim byte = 0

	for _, seg := range segmentBuffer {
		switch lastDelim {
		case ackDelimSeq:
			if isInSequence(prevSeg, seg) {
				data[len(data)-1] = ackDelimRange
				data = append(data, seg.sequenceNumber...)
				lastDelim = ackDelimRange
			} else {
				data = append(data, seg.sequenceNumber...)
				data = append(data, ackDelimSeq)
			}
		case ackDelimRange:
			if isInSequence(prevSeg, seg) {
				copy(data[len(data)-4:], seg.sequenceNumber)
			} else {
				data = append(data, ackDelimSeq)
				data = append(data, seg.sequenceNumber...)
				data = append(data, ackDelimSeq)
				lastDelim = ackDelimSeq
			}
		default:
			data = append(data, seg.sequenceNumber...)
			data = append(data, ackDelimSeq)
			lastDelim = ackDelimSeq
		}
		prevSeg = seg
	}
	if lastDelim == ackDelimSeq {
		data[len(data)-1] = ackDelimEnd
	} else {
		data = append(data, ackDelimEnd)
	}

	return createFlaggedSegment(sequenceNumber, flagACK, data)
}

func ackSegmentToSequenceNumbers(ack *segment) []uint32 {
	segs := make([]uint32, 0)
	if !ack.isFlaggedAs(flagACK) {
		return segs
	}
	segs = append(segs, ack.getSequenceNumber())
	if len(ack.data) <= 1 {
		return segs
	}

	delim := ackDelimSeq
	for i := 0; delim != ackDelimEnd && i < len(ack.data); i++ {
		switch delim {
		case ackDelimSeq:
			sequenceNum := bytesToUint32(ack.data[i : i+4])
			segs = append(segs, sequenceNum)
			i += 4
		case ackDelimRange:
			to := bytesToUint32(ack.data[i : i+4])
			i += 4
			for current := segs[len(segs)-1] + 1; current <= to; current++ {
				segs = append(segs, current)
			}
		}
		delim = ack.data[i]
	}

	return segs
}

func getNextSegmentInBuffer(currentIndex int, sequenceNum uint32, buffer []byte) (int, *segment) {
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

func hasSegmentTimedOut(seg *segment, timestamp time.Time) bool {
	if seg == nil {
		return false
	}

	timeout := seg.timestamp.Add(retransmissionTimeout)
	return timestamp.After(timeout)
}

func insertSegmentInOrder(segments []*segment, insert *segment) []*segment {
	for i, seg := range segments {
		if insert.getSequenceNumber() < seg.getSequenceNumber() {
			segments = append(segments, nil)
			copy(segments[i+1:], segments[i:])
			segments[i] = insert
			return segments
		}
		if insert.getSequenceNumber() == seg.getSequenceNumber() {
			return segments
		}
	}
	return append(segments, insert)
}

func removeSegment(segments []*segment, sequenceNumber uint32) (*segment, []*segment) {
	for i, seg := range segments {
		if seg.getSequenceNumber() == sequenceNumber {
			return seg, append(segments[:i], segments[i+1:]...)
		}
	}
	return nil, segments
}

func removeAllSegmentsWhere(segments []*segment, condition func(*segment) bool) (removed []*segment, orig []*segment) {
	removed = make([]*segment, 0, len(segments))
	for i := 0; i < len(segments); i++ {
		seg := segments[i]
		if condition(seg) {
			segments = append(segments[:i], segments[i+1:]...)
			removed = append(removed, seg)
			i--
		}
	}
	return removed, segments
}

func popSegment(segments []*segment) (*segment, []*segment) {
	return segments[0], segments[1:]
}
