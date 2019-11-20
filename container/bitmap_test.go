package container

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestBitmap_AddLinear(t *testing.T) {
	b := NewBitmap(3)

	b.Add(0, nil)
	b.Add(1, nil)

	assert.Equal(t, uint32(0), b.bitmapData[0])
	assert.Equal(t, uint32(0), b.bitmapData[1])
	assert.Equal(t, uint32(0), b.bitmapData[2])

	assert.Equal(t, uint32(2), b.SeqNumber)

}

func TestBitmap_MoveTwice(t *testing.T) {
	b := NewBitmap(3)

	b.Add(0, nil)
	b.Add(1, nil)
	b.Add(2, nil)

	assert.Equal(t, uint32(0), b.bitmapData[0])
	assert.Equal(t, uint32(0), b.bitmapData[1])
	assert.Equal(t, uint32(0), b.bitmapData[2])

	assert.Equal(t, uint32(3), b.SeqNumber)

}

func TestBitmap_AddNonLinear(t *testing.T) {
	b := NewBitmap(3)
	b.Add(0, nil)
	b.Add(2, nil)

	assert.Equal(t, uint32(0), b.bitmapData[0])
	assert.Equal(t, uint32(1), b.bitmapData[1])
	assert.Equal(t, uint32(0), b.bitmapData[2])

	assert.Equal(t, uint32(1), b.SeqNumber)
}

func TestBitmap_ToNumber(t *testing.T) {
	b := NewBitmap(3)
	b.Add(0, nil)
	b.Add(2, nil)

	assert.Equal(t, uint32(2), b.ToNumber())

	b2 := NewBitmap(3)
	b2.Add(0, nil)
	b2.Add(1, nil)

	assert.Equal(t, uint32(0), b2.ToNumber())
}

func TestBitmap_Init(t *testing.T) {
	b := NewBitmap(7)
	b.Init(123) //ob1111011

	assert.Equal(t, 7, len(b.bitmapData))
	assert.Equal(t, uint32(1), b.bitmapData[0])
	assert.Equal(t, uint32(1), b.bitmapData[1])
	assert.Equal(t, uint32(0), b.bitmapData[2])
	assert.Equal(t, uint32(1), b.bitmapData[3])
	assert.Equal(t, uint32(1), b.bitmapData[4])
	assert.Equal(t, uint32(1), b.bitmapData[5])
	assert.Equal(t, uint32(1), b.bitmapData[6])
}
