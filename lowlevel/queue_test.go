package lowlevel

import (
	"github.com/stretchr/testify/assert"
	. "go-protocol/container"
	"testing"
)

func TestQueue_EmptyQueue(t *testing.T) {
	q := NewQueue()
	if q.Dequeue() != nil {
		t.Errorf("Empty container value not == nil")
	}
	if !q.IsEmpty() {
		t.Errorf("IsEmpty() for empty container != true")
	}
}

func printTestError(t *testing.T, name string, expected, actual int) {
	t.Errorf("Expected %s() == %d, was %d", name, expected, actual)
}

func TestQueue_WithMultipleEntries(t *testing.T) {
	q := NewQueue()
	q.Enqueue(3)
	q.Enqueue(5)
	q.Enqueue(2)

	if q.IsEmpty() {
		t.Errorf("container with 3 elements shows as empty")
	}

	actual := q.Dequeue().(int)
	if actual != 3 {
		printTestError(t, "Dequeue", 3, actual)
	}
	actual = q.Dequeue().(int)
	if actual != 5 {
		printTestError(t, "Dequeue", 5, actual)
	}
	actual = q.Dequeue().(int)
	if actual != 2 {
		printTestError(t, "Dequeue", 2, actual)
	}
}

func TestQueue_PushFront(t *testing.T) {
	q := NewQueue()
	q.Enqueue(3)
	q.PushFront(5)

	if q.IsEmpty() {
		t.Errorf("container with 3 elements shows as empty")
	}

	actual := q.Dequeue().(int)
	if actual != 5 {
		printTestError(t, "Dequeue", 5, actual)
	}
	actual = q.Dequeue().(int)
	if actual != 3 {
		printTestError(t, "Dequeue", 3, actual)
	}
}

func TestQueue_PeekRemovesNothing(t *testing.T) {
	q := NewQueue()
	q.Enqueue(3)

	if q.IsEmpty() {
		t.Errorf("container with 3 elements shows as empty")
	}

	actual := q.Peek().(int)
	if actual != 3 {
		printTestError(t, "Peek", 3, actual)
	}
	actual = q.Dequeue().(int)
	if actual != 3 {
		printTestError(t, "Dequeue", 3, actual)
	}
}

func TestQueue_CheckType(t *testing.T) {
	q1 := NewQueue()
	q1.Enqueue(Segment{SequenceNumber: []byte{0, 0, 0, 1}})

	test := func() { q1.Enqueue(1) }

	assert.Panics(t, test, "")

}

func TestQueue_IsEmpty(t *testing.T) {
	q1 := NewQueue()
	q1.Enqueue(Segment{SequenceNumber: []byte{0, 0, 0, 1}})

	assert.Equal(t, false, q1.IsEmpty())
	q1.Dequeue()
	assert.Equal(t, true, q1.IsEmpty())
}

func TestQueue_IsEmpty2(t *testing.T) {
	q1 := NewQueue()
	q1.Enqueue(&Segment{SequenceNumber: []byte{0, 0, 0, 1}})

	assert.Equal(t, false, q1.IsEmpty())
	q1.Dequeue()
	assert.Equal(t, true, q1.IsEmpty())
}

func TestQueue_IsEmpty3(t *testing.T) {
	q1 := NewQueue()
	q1.Enqueue(Segment{SequenceNumber: []byte{0, 0, 0, 1}})
	q1.Enqueue(Segment{SequenceNumber: []byte{0, 0, 0, 1}})
	q1.Enqueue(Segment{SequenceNumber: []byte{0, 0, 0, 1}})
	q1.Enqueue(Segment{SequenceNumber: []byte{0, 0, 0, 1}})
	q1.Enqueue(Segment{SequenceNumber: []byte{0, 0, 0, 1}})
	q1.Enqueue(Segment{SequenceNumber: []byte{0, 0, 0, 1}})
	q1.Enqueue(Segment{SequenceNumber: []byte{0, 0, 0, 1}})

	i := 0
	for !q1.IsEmpty() {
		ele := q1.Dequeue().(Segment)
		assert.Equal(t, uint32(1), ele.GetSequenceNumber())
		i++
	}

	assert.Equal(t, 7, i)
}
