package atp

import (
	"encoding/binary"
	"time"
)

var segmentMtu = defaultMTU

const (
	ackDelimStart byte = 1
	ackDelimSeq   byte = 2
	ackDelimRange byte = 4
	ackDelimEnd   byte = 8
)

var dataOffsetPosition = position{0, 1}
var flagPosition = position{1, 2}
var sequenceNumberPosition = position{2, 6}
var windowSizePosition = position{6, 10}

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
	windowSize     []byte
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

func (seg *segment) getWindowSize() uint32 {
	return bytesToUint32(seg.windowSize)
}

func (seg *segment) setWindowSize(windowSize uint32) {
	seg.windowSize = seg.buffer[windowSizePosition.Start:windowSizePosition.End]
	binary.BigEndian.PutUint32(seg.windowSize, windowSize)
}

func (seg *segment) getDataAsString() string {
	return string(seg.data)
}

func (seg *segment) hasTimedOut(timestamp time.Time) bool {
	return timestamp.After(seg.timestamp.Add(retransmissionTimeout))
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
	dataOffset := int(buffer[dataOffsetPosition.Start])
	flag := buffer[flagPosition.Start]
	seg := &segment{
		buffer:         buffer,
		sequenceNumber: buffer[sequenceNumberPosition.Start:sequenceNumberPosition.End],
		data:           buffer[dataOffset:],
	}
	if isFlaggedAs(flag, flagACK) {
		seg.windowSize = buffer[windowSizePosition.Start:windowSizePosition.End]
	}
	return seg
}

func getDataOffsetForFlag(flag byte) int {
	if isFlaggedAs(flag, flagACK) {
		return windowSizePosition.End
	}
	return sequenceNumberPosition.End
}

func createFlaggedSegment(sequenceNumber uint32, flags byte, data []byte) *segment {
	dataOffset := getDataOffsetForFlag(flags)
	buffer := make([]byte, dataOffset+len(data))
	setDataOffset(buffer, byte(dataOffset))
	setFlags(buffer, flags)
	setSequenceNumber(buffer, sequenceNumber)
	copy(buffer[dataOffset:], data)
	return createSegment(buffer)
}

func isInSequence(seg1, seg2 uint32) bool {
	return seg1+1 == seg2
}

func createAckSegment(sequenceNumber, windowSize uint32, numBuffer []uint32) *segment {
	data := make([]byte, 0, segmentMtu)
	var prevNum uint32
	var lastDelim byte = 0
	if len(numBuffer) > 0 {
		data = append(data, ackDelimStart)
	}

	for _, num := range numBuffer {
		switch lastDelim {
		case ackDelimSeq:
			if isInSequence(prevNum, num) {
				data[len(data)-1] = ackDelimRange
				data = append(data, uint32ToBytes(num)...)
				lastDelim = ackDelimRange
			} else {
				data = append(data, uint32ToBytes(num)...)
				data = append(data, ackDelimSeq)
			}
		case ackDelimRange:
			if isInSequence(prevNum, num) {
				copy(data[len(data)-4:], uint32ToBytes(num))
			} else {
				data = append(data, ackDelimSeq)
				data = append(data, uint32ToBytes(num)...)
				data = append(data, ackDelimSeq)
				lastDelim = ackDelimSeq
			}
		default:
			data = append(data, uint32ToBytes(num)...)
			data = append(data, ackDelimSeq)
			lastDelim = ackDelimSeq
		}
		prevNum = num
	}
	if lastDelim == ackDelimSeq {
		data[len(data)-1] = ackDelimEnd
	} else {
		data = append(data, ackDelimEnd)
	}
	seg := createFlaggedSegment(sequenceNumber, flagACK, data)
	seg.setWindowSize(windowSize)
	return seg
}

func ackSegmentToSequenceNumbers(ack *segment) []uint32 {
	segs := make([]uint32, 0)
	if !ack.isFlaggedAs(flagACK) {
		return segs
	}
	if ack.data[0] == ackDelimEnd {
		return segs
	}

	delim := ackDelimSeq
	for i := 1; delim != ackDelimEnd && i < len(ack.data); i++ {
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

func insertUin32InOrder(nums []uint32, insert uint32) []uint32 {
	for i, num := range nums {
		if insert < num {
			nums = append(nums, 0)
			copy(nums[i+1:], nums[i:])
			nums[i] = insert
			return nums
		}
		if insert == num {
			return nums
		}
	}
	return append(nums, insert)
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

func popUint32(nums []uint32) (uint32, []uint32) {
	return nums[0], nums[1:]
}
