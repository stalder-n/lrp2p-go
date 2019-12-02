package atp

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestBitmap_AddLinear(t *testing.T) {
	b := newBitmap(3)

	b.Add(1, nil)
	b.Add(2, nil)

	assert.Equal(t, uint32(1), b.bitmapData[0])
	assert.Equal(t, uint32(1), b.bitmapData[1])
	assert.Equal(t, uint32(0), b.bitmapData[2])

	assert.Equal(t, uint32(1), b.seqNumber)

}

func TestBitmap_MoveTwice(t *testing.T) {
	b := newBitmap(3)

	b.Add(1, nil)
	b.Add(2, nil)
	b.Add(3, nil)

	assert.Equal(t, uint32(1), b.bitmapData[0])
	assert.Equal(t, uint32(1), b.bitmapData[1])
	assert.Equal(t, uint32(1), b.bitmapData[2])

	assert.Equal(t, uint32(1), b.seqNumber)

}

func TestBitmap_AddNonLinear(t *testing.T) {
	b := newBitmap(3)
	b.Add(1, nil)
	b.Add(3, nil)

	assert.Equal(t, uint32(1), b.bitmapData[0])
	assert.Equal(t, uint32(0), b.bitmapData[1])
	assert.Equal(t, uint32(1), b.bitmapData[2])

	assert.Equal(t, uint32(1), b.seqNumber)
}

func TestBitmap_ToNumber(t *testing.T) {
	b := newBitmap(3)
	b.Add(1, nil)
	b.Add(3, nil)

	assert.Equal(t, uint32(5), b.ToNumber())
}

func TestBitmap_Init(t *testing.T) {
	b := newBitmap(7)
	b.Init(0, 123) //ob1111011

	assert.Equal(t, 7, len(b.bitmapData))
	assert.Equal(t, uint32(1), b.bitmapData[0])
	assert.Equal(t, uint32(1), b.bitmapData[1])
	assert.Equal(t, uint32(0), b.bitmapData[2])
	assert.Equal(t, uint32(1), b.bitmapData[3])
	assert.Equal(t, uint32(1), b.bitmapData[4])
	assert.Equal(t, uint32(1), b.bitmapData[5])
	assert.Equal(t, uint32(1), b.bitmapData[6])
}
