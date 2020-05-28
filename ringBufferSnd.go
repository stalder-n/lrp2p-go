package atp

import (
	"fmt"
	"time"
)

type ringBufferSnd struct {
	buffer     []*segment
	size       uint32
	r          uint32
	w          uint32
	timeoutSec int
	full       bool
	prevSn     uint32
}

func NewRingBufferSnd(size uint32, timeoutSec int) *ringBufferSnd {
	return &ringBufferSnd{
		buffer:     make([]*segment, size+1),
		size:       size + 1,
		timeoutSec: timeoutSec,
		prevSn:     uint32(0xffffffff), // -1 % size
	}
}

func (ring *ringBufferSnd) insertSequence(seg *segment) error {
	if ring.full { //is full
		return fmt.Errorf("ring buffer is full, cannot add %v/%v", ring.w, ring.r)
	}
	if ring.prevSn != seg.getSequenceNumber()-1 {
		return fmt.Errorf("not a sequence, cannot add %v/%v", ring.prevSn, (seg.getSequenceNumber() - 1))
	}

	ring.prevSn = seg.getSequenceNumber()
	ring.buffer[ring.w] = seg
	ring.w = (ring.w + 1) % ring.size
	if ((ring.w + 1) % ring.size) == ring.r {
		ring.full = true
	}
	return nil
}

func (ring *ringBufferSnd) getTimedout(now time.Time) []*segment {
	var ret []*segment
	var i uint32
	for ; i < ring.size; i++ {
		index := (ring.r + i) % ring.size
		seg := ring.buffer[index]
		if seg != nil {
			if seg.timestamp.Add(time.Second * time.Duration(ring.timeoutSec)).Before(now) {
				ret = append(ret, seg)
			} else {
				break
			}
		}

		if ring.w == index {
			break
		}
	}

	return ret
}

func (ring *ringBufferSnd) remove(sequenceNumber uint32) (*segment, error) {
	index := sequenceNumber % ring.size
	seg := ring.buffer[index]
	if seg == nil {
		return nil, fmt.Errorf("already removed %v", index)
	}
	if sequenceNumber != seg.getSequenceNumber() {
		return nil, fmt.Errorf("sn mismatch %v/%v", sequenceNumber, seg.getSequenceNumber())
	}
	ring.buffer[index] = nil

	for i := ring.r; i != ring.w; i = (i + 1) % ring.size {
		if ring.buffer[i] == nil {
			ring.r = (i + 1) % ring.size
		} else {
			break
		}
	}
	ring.full = false
	return seg, nil
}

func (ring *ringBufferSnd) timoutSec() int {
	return ring.timeoutSec
}
