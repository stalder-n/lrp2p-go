package atp

type bitmap struct {
	bitmapData []uint32
	data       []interface{}
	seqNumber  uint32
}

func newBitmap(size uint32) *bitmap {
	result := &bitmap{}
	result.bitmapData = make([]uint32, size)
	result.data = make([]interface{}, size)

	return result
}

func (m *bitmap) Add(seqNr uint32, data interface{}) {
	if m.seqNumber == 0 {
		m.seqNumber = seqNr
	}

	index := seqNr - m.seqNumber
	m.bitmapData[index] = 1
	m.data[index] = data
}

func (m *bitmap) Slide() {
	for m.SlideOne() {
	}
}

func (m *bitmap) SlideOne() bool {
	if m.bitmapData[0] == 1 {
		m.seqNumber++
		for i := 0; i < len(m.bitmapData)-1; i++ {
			m.bitmapData[i] = m.bitmapData[i+1]
			if m.data != nil {
				m.data[i] = m.data[i+1]
			}
		}
		m.bitmapData[len(m.bitmapData)-1] = 0
		return true
	}
	return false
}

func (m *bitmap) Get(seqNr uint32) (bit uint32, data interface{}) {
	bit = m.bitmapData[seqNr]
	if m.data != nil {
		data = m.data[seqNr]
	}
	return
}

func (m *bitmap) ToNumber() uint32 {
	result := uint32(0)
	for i := 0; i < len(m.bitmapData); i++ {
		ele, _ := m.Get(uint32(i))
		result += ele * (1 << i)
	}

	return result
}

func (m *bitmap) Init(seqNr uint32, bitmap uint32) *bitmap {
	i := uint32(0)
	for num := uint32(0); bitmap != 0; bitmap = bitmap / 2 {
		num = bitmap % 2
		m.bitmapData[i] = num

		i++
	}

	m.seqNumber = seqNr

	return m
}

func (m *bitmap) GetAndRemoveInorder() interface{} {
	if m.bitmapData[0] == uint32(1) {
		seg := m.data[0]
		m.SlideOne()
		return seg
	} else {
		return nil
	}
}
