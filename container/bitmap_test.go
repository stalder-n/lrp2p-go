package container

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestBitmap_Set(t *testing.T) {
	b := New(3)

	b.Set(0, 1)
	b.Set(1, 0)
	b.Set(2, 3)

	assert.Equal(t, uint32(1), b.data[0])
	assert.Equal(t, uint32(0), b.data[1])
	assert.Equal(t, uint32(1), b.data[2])

	b.Set(0, 0)
	assert.Equal(t, uint32(0), b.data[0])

}

func TestBitmap_Get(t *testing.T) {
	b := New(3)
	b.Set(0, 1)
	b.Set(1, 0)
	b.Set(2, 3)

	assert.Equal(t, uint32(1), b.Get(0))
	assert.Equal(t, uint32(0), b.Get(1))
	assert.Equal(t, uint32(1), b.Get(2))
}

func TestBitmap_ToNumber(t *testing.T) {
	b := New(3)
	b.Set(0, 1)
	b.Set(1, 0)
	b.Set(2, 1)

	assert.Equal(t, uint32(5), b.ToNumber())

	b2 := New(3)
	b2.Set(0, 1)
	b2.Set(1, 1)
	b2.Set(2, 0)

	assert.Equal(t, uint32(3), b2.ToNumber())
}

func TestBitmap_Init(t *testing.T) {
	b := New(7)
	b.Init(123) //ob1111011

	assert.Equal(t, 7, len(b.data))
	assert.Equal(t, uint32(1), b.data[0])
	assert.Equal(t, uint32(1), b.data[1])
	assert.Equal(t, uint32(0), b.data[2])
	assert.Equal(t, uint32(1), b.data[3])
	assert.Equal(t, uint32(1), b.data[4])
	assert.Equal(t, uint32(1), b.data[5])
	assert.Equal(t, uint32(1), b.data[6])
}

func TestBitmap_ToNumberInverted(t *testing.T) {
	b1 := New(7)
	b1.Init(123) //ob1111011

	b2 := New(7)
	b2.Init(4) //ob0000100

	for i := 0; i < 7; i++ {
		a := b1.data[i]
		b := b2.data[i]
		assert.NotEqual(t, a, b)
	}

	assert.Equal(t, b1.ToNumber(), b2.ToNumberInverted())

}
