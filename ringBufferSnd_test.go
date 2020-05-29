package atp

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestInsert(t *testing.T) {
	r := NewRingBufferSnd(10, 3)
	seg := makeSegment(0)
	err := r.insertSequence(seg)
	assert.NoError(t, err)
}

func TestInsertNotOrdered(t *testing.T) {
	r := NewRingBufferSnd(10, 3)
	seg := makeSegment(0)
	err := r.insertSequence(seg)
	assert.NoError(t, err)
	seg = makeSegment(2)
	err = r.insertSequence(seg)
	assert.Error(t, err)
}

func TestNotOrdered(t *testing.T) {
	r := NewRingBufferSnd(10, 3)
	seg := makeSegment(1)
	err := r.insertSequence(seg)
	assert.Error(t, err)
}

func TestFullSnd(t *testing.T) {
	r := NewRingBufferSnd(10, 3)
	for i := 0; i < 10; i++ {
		seg := makeSegment(uint32(i))
		err := r.insertSequence(seg)
		assert.NoError(t, err)
	}
	seg := makeSegment(11)
	err := r.insertSequence(seg)
	assert.Error(t, err)
}

func TestRemove(t *testing.T) {
	r := NewRingBufferSnd(10, 3)
	for i := 0; i < 10; i++ {
		seg := makeSegment(uint32(i))
		err := r.insertSequence(seg)
		assert.NoError(t, err)
	}
	r.remove(5)
	s := r.getTimedout(timeZero.Add(time.Second*time.Duration(r.timoutSec()) + 1))
	assert.Equal(t, 9, len(s))
}

func TestRemove5(t *testing.T) {
	r := NewRingBufferSnd(10, 3)
	for i := 0; i < 5; i++ {
		seg := makeSegment(uint32(i))
		err := r.insertSequence(seg)
		assert.NoError(t, err)
	}
	r.remove(4)
	s := r.getTimedout(timeZero.Add(time.Second*time.Duration(r.timoutSec()) + 1))
	assert.Equal(t, 4, len(s))
}

func TestNoRemove(t *testing.T) {
	r := NewRingBufferSnd(10, 3)
	for i := 0; i < 5; i++ {
		seg := makeSegment(uint32(i))
		err := r.insertSequence(seg)
		assert.NoError(t, err)
	}
	r.remove(4)
	//no timeout yet
	s := r.getTimedout(timeZero.Add(time.Second * time.Duration(r.timoutSec())))
	assert.Equal(t, 0, len(s))
}

func TestInsertRemove(t *testing.T) {
	r := NewRingBufferSnd(10, 3)

	for i := 0; i < 5; i++ {
		seg := makeSegment(uint32(i))
		err := r.insertSequence(seg)
		assert.NoError(t, err)
	}
	_, err := r.remove(3)
	assert.NoError(t, err)
	_, err = r.remove(1)
	assert.NoError(t, err)

	s := r.getTimedout(timeZero)
	assert.Equal(t, 0, len(s))
	s = r.getTimedout(timeZero.Add(time.Second*time.Duration(r.timoutSec()) + 1))
	assert.Equal(t, 3, len(s))
}

func TestInsertRemove2(t *testing.T) {
	r := NewRingBufferSnd(10, 3)
	seg := makeSegment(0)
	err := r.insertSequence(seg)
	assert.NoError(t, err)
	_, err = r.remove(0)
	assert.NoError(t, err)
	err = r.insertSequence(seg)
	assert.Error(t, err)
	s := r.getTimedout(timeZero)
	assert.Equal(t, 0, len(s))
	s = r.getTimedout(timeZero.Add(time.Second*time.Duration(r.timoutSec()) + 1))
	assert.Equal(t, 0, len(s))
}

func TestAlmostFull(t *testing.T) {
	r := NewRingBufferSnd(10, 3)
	for i := 0; i < 10; i++ {
		seg := makeSegment(uint32(i))
		err := r.insertSequence(seg)
		assert.NoError(t, err)
	}
	r.remove(4)
	seg := makeSegment(10)
	err := r.insertSequence(seg)
	assert.Error(t, err)
	r.remove(0)

	seg = makeSegment(10)
	err = r.insertSequence(seg)
	assert.NoError(t, err)
}
