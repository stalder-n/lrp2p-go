package container

type Bitmap struct {
	data []uint32
}

func (m *Bitmap) New(size int) {
	m.data = make([]uint32, size)
}

func (m *Bitmap) Set(index uint32, value uint32) {
	if value >= 1 {
		value = 1
	}

	m.data[index] = value
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
	i := uint32(0);
	for num := uint32(0); number != 0; number = number / 2 {
		num = number % 2
		m.Set(i, num)
		i++
	}
}
