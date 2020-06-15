package atp

import (
	"fmt"
	"time"
)

type ringBufferSnd struct {
	buffer     []*segment
	s          uint32
	r          uint32
	w          uint32
	timeoutSec int
	prevSn     uint32
	old        *ringBufferSnd
	newSize    uint32
}

func NewRingBufferSnd(size uint32, timeoutSec int) *ringBufferSnd {
	return &ringBufferSnd{
		buffer:     make([]*segment, size+1),
		s:          size + 1,
		timeoutSec: timeoutSec,
		prevSn:     uint32(0xffffffff), // -1
	}
}

func (ring *ringBufferSnd) size() uint32 {
	return ring.s - 1
}

func (ring *ringBufferSnd) resize(targetSize uint32) (bool, *ringBufferSnd) {
	if targetSize == ring.size() || ring.old != nil {
		return false, ring
	} else {
		r := NewRingBufferSnd(targetSize, ring.timeoutSec)
		r.old = ring
		r.prevSn = ring.prevSn
		r.w = (r.prevSn + 1) % r.s
		r.r = r.w
		return true, r
	}
}

func (ring *ringBufferSnd) insertSequence(seg *segment) (bool, error) {
	if ((ring.w + 1) % ring.s) == ring.r { //is full
		return false, nil
	}
	if ring.prevSn != seg.getSequenceNumber()-1 {
		return false, fmt.Errorf("not a sequence, cannot add %v/%v", ring.prevSn, (seg.getSequenceNumber() - 1))
	}
	if ring.buffer[ring.w] != nil {
		return false, fmt.Errorf("not empty at pos %v", ring.w)
	}
	ring.prevSn = seg.getSequenceNumber()
	ring.buffer[ring.w] = seg
	ring.w = (ring.w + 1) % ring.s
	return true, nil
}

func (ring *ringBufferSnd) getTimedout(now time.Time) []*segment {
	var ret []*segment
	if ring.old != nil {
		ret = ring.old.getTimedout(now)
	}

	for i := uint32(0); i < ring.s; i++ {
		index := (ring.r + i) % ring.s
		seg := ring.buffer[index]
		if seg != nil {
			if seg.timestamp.Add(time.Second * time.Duration(ring.timeoutSec)).Before(now) {
				ret = append(ret, seg)
			}
		}

		if ring.w == index {
			break
		}
	}

	return ret
}

func (ring *ringBufferSnd) remove(sequenceNumber uint32) (*segment, bool, error) {
	if ring.old != nil {
		seg, empty, err := ring.old.remove(sequenceNumber)
		if empty {
			ring.old = nil
			empty = false
		}
		if err == nil {
			return seg, empty, nil
		}
	}
	index := sequenceNumber % ring.s
	seg := ring.buffer[index]
	if seg == nil {
		return nil, false, fmt.Errorf("already removed %v", index)
	}
	if sequenceNumber != seg.getSequenceNumber() {
		return nil, false, fmt.Errorf("sn mismatch %v/%v", sequenceNumber, seg.getSequenceNumber())
	}
	ring.buffer[index] = nil

	empty := true
	for i := ring.r; i != ring.w; i = (i + 1) % ring.s {
		if ring.buffer[i] == nil {
			ring.r = (i + 1) % ring.s
		} else {
			empty = false
			break
		}
	}
	return seg, empty, nil
}

func (ring *ringBufferSnd) timoutSec() int {
	return ring.timeoutSec
}
