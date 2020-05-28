package atp

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestInsertOutOfOrder(t *testing.T) {
	r := NewRingBufferRcv(10)
	seg := makeSegment(1)
	err := r.insert(seg)
	assert.NoError(t, err)

	s := r.removeSequence()
	assert.Equal(t, len(s), 0)
}

func TestInsertOutOfOrder2(t *testing.T) {
	r := NewRingBufferRcv(10)
	seg := makeSegment(1)
	err := r.insert(seg)
	assert.NoError(t, err)

	seg = makeSegment(0)
	err = r.insert(seg)
	assert.NoError(t, err)

	s := r.removeSequence()
	assert.Equal(t, len(s), 2)
}

func TestInsertFull(t *testing.T) {
	r := NewRingBufferRcv(10)
	for i := 0; i < 10; i++ {
		seg := makeSegment(uint32(i))
		err := r.insert(seg)
		assert.NoError(t, err)
	}

	seg := makeSegment(11)
	err := r.insert(seg)
	assert.Error(t, err)
}

func TestInsertBackwards(t *testing.T) {
	r := NewRingBufferRcv(10)
	for i := 0; i < 9; i++ {
		seg := makeSegment(uint32(9 - i))
		err := r.insert(seg)
		assert.NoError(t, err)
	}
	s := r.removeSequence()
	assert.Equal(t, len(s), 0)

	seg := makeSegment(0)
	err := r.insert(seg)
	assert.NoError(t, err)

	s = r.removeSequence()
	assert.Equal(t, len(s), 10)

}

func TestInsertTwice(t *testing.T) {
	r := NewRingBufferRcv(10)
	seg := makeSegment(1)
	err := r.insert(seg)
	assert.NoError(t, err)
	seg = makeSegment(1)
	err = r.insert(seg)
	assert.Error(t, err)
}

func TestFull(t *testing.T) {
	r := NewRingBufferRcv(10)

	for i := 0; i < 10; i++ {
		seg := makeSegment(uint32(i))
		err := r.insert(seg)
		assert.NoError(t, err)
	}

	seg := makeSegment(uint32(11))
	err := r.insert(seg)

	assert.Error(t, err)
}

func TestModulo(t *testing.T) {
	r := NewRingBufferRcv(10)

	for i := 0; i < 10; i++ {
		seg := makeSegment(uint32(i))
		err := r.insert(seg)
		assert.NoError(t, err)
	}

	s := r.removeSequence()
	assert.Equal(t, len(s), 10)

	for i := 10; i < 20; i++ {
		seg := makeSegment(uint32(i))
		err := r.insert(seg)
		assert.NoError(t, err)
	}

	s = r.removeSequence()
	assert.Equal(t, len(s), 10)
}

func TestWrongSN(t *testing.T) {
	r := NewRingBufferRcv(10)
	seg := makeSegment(1)
	err := r.insert(seg)
	assert.NoError(t, err)
	seg = makeSegment(11)
	err = r.insert(seg)
	assert.Error(t, err)
}

func makeSegment(data uint32) *segment {
	return &segment{
		buffer:         nil,
		sequenceNumber: uint32ToBytes(data),
		windowSize:     nil,
		data:           nil,
		timestamp:      time.Time{},
	}
}
