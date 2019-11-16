package container

import "strconv"

type Bitmap struct {
	data []uint32
	len  uint32
}

func New(size uint32) *Bitmap {
	result := &Bitmap{}
	result.data = make([]uint32, size)

	return result
}

func (m *Bitmap) Add(value uint32) {
	index := value % uint32(len(m.data))

	m.data[index] = 1
	m.len++
}

func (m *Bitmap) Remove(value uint32) {
	index := value % uint32(len(m.data))

	m.data[index] = 0
	m.len--
}

func (m *Bitmap) Get(index uint32) uint32 {
	return m.data[index]
}

func (m *Bitmap) ToNumber() uint32 {
	result := uint32(0)
	for i := 0; i < len(m.data); i++ {
		ele := m.Get(uint32(i))
		result += ele * (1 << i)
	}

	return result
}

func (m *Bitmap) ToNumberInverted() uint32 {
	result := uint32(0)
	for i := 0; i < len(m.data); i++ {
		ele := m.Get(uint32(i))
		ele = ^ele & 1 //NOT
		result += ele * (1 << i)
	}

	return result
}

func (m *Bitmap) Init(number uint32) {
	i := uint32(0)
	for num := uint32(0); number != 0; number = number / 2 {
		num = number % 2
		m.set(i, num)

		i++
	}

	m.len = i
}

func (m *Bitmap) set(index uint32, value uint32) {

	if index >= uint32(len(m.data)) {
		panic("index is out of range: " + strconv.FormatUint(uint64(index), 10) + " len: " + strconv.Itoa(len(m.data)))
	}

	m.data[index] = value
}

func (m *Bitmap) Len() uint32 {
	return m.len
}
