package atp

import "fmt"

//size of RTO could be measured by RTT and bandwidth
//https://www.sciencedirect.com/topics/computer-science/maximum-window-size

type ringBufferRcv struct {
	buffer []*segment
	s      uint32
	r      uint32
	minSn  uint32
	old    []*segment
}

func NewRingBufferRcv(size uint32) *ringBufferRcv {
	return &ringBufferRcv{
		buffer: make([]*segment, size),
		s:      size,
	}
}

func (ring *ringBufferRcv) size() uint32 {
	return ring.s
}

func (ring *ringBufferRcv) resize(targetSize uint32) *ringBufferRcv {
	if targetSize == ring.s {
		return ring
	} else {
		r := NewRingBufferRcv(targetSize)
		r.old = make([]*segment, ring.s)
		i := 0
		for _, v := range ring.buffer {
			err := r.insert(v)
			if err != nil {
				r.old[i] = v
				i++
			}
			r.old = r.old[:i]
		}
		return r
	}
}

func (ring *ringBufferRcv) insert(seg *segment) error {
	sn := seg.getSequenceNumber()
	if sn > (ring.minSn + ring.s) { //is full
		return fmt.Errorf("ring buffer is full, cannot add %v/%v", ring.minSn, sn)
	}
	index := sn % ring.s
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
	for ; i < ring.s; i++ {
		seg := ring.buffer[ring.r]
		if seg != nil {
			ring.buffer[ring.r] = nil
			ret = append(ret, seg)
			ring.r = (ring.r + 1) % ring.s
			ring.minSn = seg.getSequenceNumber()
			ring.drainOverflow()
		} else {
			break
		}
	}
	return ret
}

func (ring *ringBufferRcv) drainOverflow() {
	if len(ring.old) > 0 {
		err := ring.insert(ring.old[0])
		if err == nil {
			ring.old = ring.old[1:]
		}
	} else {
		ring.old = nil
	}
}
