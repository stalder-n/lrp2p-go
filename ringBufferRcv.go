package atp

//size of RTO could be measured by RTT and bandwidth
//https://www.sciencedirect.com/topics/computer-science/maximum-window-size

type ringBufferRcv struct {
	buffer    []*segment
	s         uint32
	r         uint32
	minGoodSn uint32
	old       []*segment
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
		r.minGoodSn = ring.minGoodSn
		r.r = (r.minGoodSn + 1) % r.s
		r.old = ring.buffer
		j := 0

		for i := uint32(0); i < ring.s; i++ {
			if ring.buffer[i] == nil {
				continue
			}
			inserted := r.insert(ring.buffer[i])
			if !inserted {
				r.old[j] = ring.buffer[i]
				j++
			}
		}
		r.old = r.old[:j]
		return r
	}
}

func (ring *ringBufferRcv) insert(seg *segment) bool {
	sn := seg.getSequenceNumber()
	if sn > (ring.minGoodSn + ring.s) { //is full cannot add data beyond the size of the buffer
		return false
	}
	if sn < diffMin(ring.minGoodSn, ring.s) { //late packet, we can ignore it
		return false
	}
	index := sn % ring.s
	//we may receive a duplicate, don't add
	if ring.buffer[index] != nil {
		return false
	}
	ring.buffer[index] = seg
	return true
}

func (ring *ringBufferRcv) removeSequence(removeMultiple bool) []*segment {
	//fast path
	if ring.buffer[ring.r] == nil {
		return nil
	}

	var ret []*segment
	for i := ring.r; i < ring.s; i++ {
		seg := ring.buffer[ring.r]
		if seg != nil {
			ring.buffer[ring.r] = nil
			ret = append(ret, seg)
			ring.r = (ring.r + 1) % ring.s
			ring.minGoodSn = seg.getSequenceNumber()
			ring.drainOverflow()
			if !removeMultiple {
				return ret
			}
		} else {
			break
		}
	}
	return ret
}

func (ring *ringBufferRcv) drainOverflow() {
	if len(ring.old) > 0 {
		inserted := ring.insert(ring.old[0])
		if inserted {
			ring.old = ring.old[1:]
		}
	} else {
		ring.old = nil
	}
}

func diffMin(x uint32, y uint32) uint32 {
	if x < y {
		return 0
	}
	return x - y
}
