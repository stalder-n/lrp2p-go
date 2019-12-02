package goprotocol

type Bitmap struct {
	bitmapData []uint32
	data       []interface{}
	SeqNumber  uint32
}

func NewBitmap(size uint32) *Bitmap {
	result := &Bitmap{}
	result.bitmapData = make([]uint32, size)
	result.data = make([]interface{}, size)

	return result
}

func (m *Bitmap) Add(seqNr uint32, data interface{}) {
	if m.SeqNumber == 0 {
		m.SeqNumber = seqNr
	}

	index := seqNr - m.SeqNumber
	m.bitmapData[index] = 1
	m.data[index] = data
}

func (m *Bitmap) Slide() {
	for m.SlideOne() {
	}
}

func (m *Bitmap) SlideOne() bool {
	if m.bitmapData[0] == 1 {
		m.SeqNumber++
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

func (m *Bitmap) Get(seqNr uint32) (bit uint32, data interface{}) {
	bit = m.bitmapData[seqNr]
	if m.data != nil {
		data = m.data[seqNr]
	}
	return
}

func (m *Bitmap) ToNumber() uint32 {
	result := uint32(0)
	for i := 0; i < len(m.bitmapData); i++ {
		ele, _ := m.Get(uint32(i))
		result += ele * (1 << i)
	}

	return result
}

func (m *Bitmap) Init(seqNr uint32, bitmap uint32) *Bitmap {
	i := uint32(0)
	for num := uint32(0); bitmap != 0; bitmap = bitmap / 2 {
		num = bitmap % 2
		m.bitmapData[i] = num

		i++
	}

	m.SeqNumber = seqNr

	return m
}

func (m *Bitmap) GetAndRemoveInorder() interface{} {
	if m.bitmapData[0] == uint32(1) {
		seg := m.data[0]
		m.SlideOne()
		return seg
	} else {
		return nil
	}
}
