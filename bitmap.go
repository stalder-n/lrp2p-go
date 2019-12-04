package atp

type bitmap struct {
	bitmapData     []uint32
	data           []*segment
	sequenceNumber uint32
}

func newEmptyBitmap(size uint32) *bitmap {
	bmap := &bitmap{}
	bmap.bitmapData = make([]uint32, size)
	bmap.data = make([]*segment, size)
	return bmap
}

func newBitmap(size, sequenceNumber, initialBitmap uint32) *bitmap {
	bmap := newEmptyBitmap(size)
	bmap.sequenceNumber = sequenceNumber
	bmap.init(initialBitmap)
	return bmap
}

func (m *bitmap) init(bmap uint32) {
	for i := 0; bmap != 0; i++ {
		var mod uint32
		bmap, mod = bitmapDiff(bmap)
		m.bitmapData[i] = mod
	}
}

func bitmapDiff(bitmap uint32) (uint32, uint32) {
	return bitmap / 2, bitmap % 2
}

func (m *bitmap) Add(sequenceNumber uint32, data *segment) {
	if sequenceNumber < m.sequenceNumber {
		return
	}
	if m.sequenceNumber == 0 {
		m.sequenceNumber = sequenceNumber
	}

	index := sequenceNumber - m.sequenceNumber
	m.bitmapData[index] = 1
	m.data[index] = data
}

func (m *bitmap) Slide() {
	for m.bitmapData[0] == 1 {
		m.slideOne()
	}
}

func (m *bitmap) slideOne() {
	m.sequenceNumber++
	copy(m.bitmapData, m.bitmapData[1:])
	m.bitmapData[len(m.bitmapData)-1] = 0
}

func (m *bitmap) Get(index int) (bit uint32, data *segment) {
	bit = m.bitmapData[index]
	if m.data != nil {
		data = m.data[index]
	}
	return
}

func (m *bitmap) ToNumber() uint32 {
	var result uint32
	for i := 0; i < len(m.bitmapData); i++ {
		bit := m.bitmapData[i]
		result |= bit << i
	}

	return result
}

func (m *bitmap) GetAndRemoveInorder() *segment {
	if m.bitmapData[0] == 1 {
		seg := m.data[0]
		m.slideOne()
		return seg
	}
	return nil
}
