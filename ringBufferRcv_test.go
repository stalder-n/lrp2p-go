package lrp2p

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"testing"
	"time"
)

func TestInsertOutOfOrder(t *testing.T) {
	r := NewRingBufferRcv(10)
	seg := makeSegment(1)
	inserted := r.insert(seg)
	assert.True(t, inserted)

	s := r.removeSequence(true)
	assert.Equal(t, len(s), 0)
}

func TestInsertOutOfOrder2(t *testing.T) {
	r := NewRingBufferRcv(10)
	seg := makeSegment(1)
	inserted := r.insert(seg)
	assert.True(t, inserted)

	seg = makeSegment(0)
	inserted = r.insert(seg)
	assert.True(t, inserted)

	s := r.removeSequence(true)
	assert.Equal(t, len(s), 2)
}

func TestInsertBackwards(t *testing.T) {
	r := NewRingBufferRcv(10)
	for i := 0; i < 9; i++ {
		seg := makeSegment(uint32(9 - i))
		inserted := r.insert(seg)
		assert.True(t, inserted)
	}
	s := r.removeSequence(true)
	assert.Equal(t, len(s), 0)

	seg := makeSegment(0)
	inserted := r.insert(seg)
	assert.True(t, inserted)

	s = r.removeSequence(true)
	assert.Equal(t, len(s), 10)

}

func TestInsertTwice(t *testing.T) {
	r := NewRingBufferRcv(10)
	seg := makeSegment(1)
	inserted := r.insert(seg)
	assert.True(t, inserted)
	seg = makeSegment(1)
	inserted = r.insert(seg)
	assert.False(t, inserted)
}

func TestFull(t *testing.T) {
	r := NewRingBufferRcv(10)

	for i := 0; i < 10; i++ {
		seg := makeSegment(uint32(i))
		inserted := r.insert(seg)
		assert.True(t, inserted)
	}

	seg := makeSegment(uint32(11))
	inserted := r.insert(seg)
	assert.False(t, inserted)
}

func TestModulo(t *testing.T) {
	r := NewRingBufferRcv(10)

	for i := 0; i < 10; i++ {
		seg := makeSegment(uint32(i))
		inserted := r.insert(seg)
		assert.True(t, inserted)
	}

	s := r.removeSequence(true)
	assert.Equal(t, len(s), 10)

	for i := 10; i < 20; i++ {
		seg := makeSegment(uint32(i))
		inserted := r.insert(seg)
		assert.True(t, inserted)
	}

	s = r.removeSequence(true)
	assert.Equal(t, len(s), 10)
}

func TestWrongSN(t *testing.T) {
	r := NewRingBufferRcv(10)
	seg := makeSegment(1)
	inserted := r.insert(seg)
	assert.True(t, inserted)
	seg = makeSegment(2)
	inserted = r.insert(seg)
	inserted = r.insert(seg)
	assert.False(t, inserted)
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

func TestFuzz2(t *testing.T) {
	r := NewRingBufferRcv(10)

	seqIns := 0
	seqRem := 0
	rand.Seed(42)

	for j := 0; j < 10000; j++ {
		rnd := rand.Intn(int(r.s)) + 1

		j := 0
		for i := rnd - 1; i >= 0; i-- {
			seg := makeSegment(uint32(seqIns + i))
			inserted := r.insert(seg)
			if inserted {
				j++
			}
		}
		seqIns += j

		s := r.removeSequence(true)

		if rand.Intn(3) == 0 {
			r = r.resize(r.size() + 1)
		}
		fmt.Printf("removed: %v\n", len(s))
	}
	fmt.Printf("send %v, recv %v", seqIns, seqRem)
}
