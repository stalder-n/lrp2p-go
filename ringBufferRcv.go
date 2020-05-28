package atp

import "fmt"

type ringBufferRcv struct {
	buffer []*segment
	size   uint32
	r      uint32
	minSn  uint32
}

func NewRingBufferRcv(size uint32) *ringBufferRcv {
	return &ringBufferRcv{
		buffer: make([]*segment, size),
		size:   size,
	}
}

func (ring *ringBufferRcv) insert(seg *segment) error {
	sn := seg.getSequenceNumber()
	if sn > (ring.minSn + ring.size) { //is full
		return fmt.Errorf("ring buffer is full, cannot add %v/%v", ring.minSn, sn)
	}
	index := sn % ring.size
	if ring.buffer[index] != nil {
		return fmt.Errorf("overwriting ringbuffer, check your logic %v/%v", sn, index)
	}
	ring.buffer[index] = seg
	return nil
}

func (ring *ringBufferRcv) removeSequence() []*segment {
	//fast path
	if ring.buffer[ring.r] == nil {
		return nil
	}

	var ret []*segment
	var i uint32
	for ; i < ring.size; i++ {
		seg := ring.buffer[ring.r]
		if seg != nil {
			ring.buffer[ring.r] = nil
			ret = append(ret, seg)
			ring.r = (ring.r + 1) % ring.size
			ring.minSn = seg.getSequenceNumber()
		} else {
			break
		}
	}
	return ret
}
