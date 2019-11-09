package protocol

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestQueue_EmptyQueue(t *testing.T) {
	q := Queue{}
	if q.Dequeue() != nil {
		t.Errorf("Empty Queue value not == nil")
	}
	if !q.IsEmpty() {
		t.Errorf("IsEmpty() for empty Queue != true")
	}
}

func printTestError(t *testing.T, name string, expected, actual int) {
	t.Errorf("Expected %s() == %d, was %d", name, expected, actual)
}

func TestQueue_WithMultipleEntries(t *testing.T) {
	q := Queue{}
	q.Enqueue(3)
	q.Enqueue(5)
	q.Enqueue(2)

	if q.IsEmpty() {
		t.Errorf("Queue with 3 elements shows as empty")
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
	q := Queue{}
	q.Enqueue(3)
	q.PushFront(5)

	if q.IsEmpty() {
		t.Errorf("Queue with 3 elements shows as empty")
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
	q := Queue{}
	q.Enqueue(3)

	if q.IsEmpty() {
		t.Errorf("Queue with 3 elements shows as empty")
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

func TestQueue_PushFrontList(t *testing.T) {
	q := Queue{}
	q.Enqueue(1)
	q.Enqueue(2)
	q.Enqueue(3)
	q.Enqueue(4)
	q.Enqueue(5)
	q.Enqueue(6)

	test := Queue{}
	test.PushFrontList(&q.list)

	for !q.IsEmpty() {
		ele1 := q.Dequeue();
		ele2 := test.Dequeue();
		assert.Equal(t, ele1, ele2)
	}
}

func TestQueue_GetElementGreaterSequenceNumber(t *testing.T) {
	q1 := &Queue{}
	q1.Enqueue(segment{sequenceNumber: []byte{0, 0, 0, 1}})
	q1.Enqueue(segment{sequenceNumber: []byte{0, 0, 0, 4}})
	q1.Enqueue(segment{sequenceNumber: []byte{0, 0, 0, 2}})
	q1.Enqueue(segment{sequenceNumber: []byte{0, 0, 0, 5}})
	q1.Enqueue(segment{sequenceNumber: []byte{0, 0, 0, 3}})
	q1.Enqueue(segment{sequenceNumber: []byte{0, 0, 0, 6}})

	q2 := Queue{}
	q2.Enqueue(segment{sequenceNumber: []byte{0, 0, 0, 4}})
	q2.Enqueue(segment{sequenceNumber: []byte{0, 0, 0, 5}})
	q2.Enqueue(segment{sequenceNumber: []byte{0, 0, 0, 6}})

	l2 := q1.GetElementGreaterSequenceNumber(3)

	for ele := l2.Front(); ele != nil; ele = ele.Next() {
		expected := q2.Dequeue().(segment)
		test := ele.Value.(segment)

		assert.Equal(t, expected.getSequenceNumber(), test.getSequenceNumber());
	}
}

func TestQueue_GetElementsGreaterOrEqualsSequenceNumber(t *testing.T) {
	q1 := &Queue{}
	q1.Enqueue(segment{sequenceNumber: []byte{0, 0, 0, 1}})
	q1.Enqueue(segment{sequenceNumber: []byte{0, 0, 0, 4}})
	q1.Enqueue(segment{sequenceNumber: []byte{0, 0, 0, 2}})
	q1.Enqueue(segment{sequenceNumber: []byte{0, 0, 0, 5}})
	q1.Enqueue(segment{sequenceNumber: []byte{0, 0, 0, 3}})
	q1.Enqueue(segment{sequenceNumber: []byte{0, 0, 0, 6}})

	q2 := Queue{}
	q2.Enqueue(segment{sequenceNumber: []byte{0, 0, 0, 3}})
	q2.Enqueue(segment{sequenceNumber: []byte{0, 0, 0, 4}})
	q2.Enqueue(segment{sequenceNumber: []byte{0, 0, 0, 5}})
	q2.Enqueue(segment{sequenceNumber: []byte{0, 0, 0, 6}})

	l2 := q1.GetElementsGreaterOrEqualsSequenceNumber(3)

	for ele := l2.Front(); ele != nil; ele = ele.Next() {
		expected := q2.Dequeue().(segment)
		test := ele.Value.(segment)

		assert.Equal(t, expected.getSequenceNumber(), test.getSequenceNumber());
	}

}

func TestQueue_GetElementsSmallerSequenceNumber(t *testing.T) {
	q1 := &Queue{}
	q1.Enqueue(segment{sequenceNumber: []byte{0, 0, 0, 1}})
	q1.Enqueue(segment{sequenceNumber: []byte{0, 0, 0, 4}})
	q1.Enqueue(segment{sequenceNumber: []byte{0, 0, 0, 2}})
	q1.Enqueue(segment{sequenceNumber: []byte{0, 0, 0, 5}})
	q1.Enqueue(segment{sequenceNumber: []byte{0, 0, 0, 3}})
	q1.Enqueue(segment{sequenceNumber: []byte{0, 0, 0, 6}})

	q2 := Queue{}
	q2.Enqueue(segment{sequenceNumber: []byte{0, 0, 0, 1}})
	q2.Enqueue(segment{sequenceNumber: []byte{0, 0, 0, 2}})

	l2 := q1.GetElementsSmallerSequenceNumber(3)

	for ele := l2.Front(); ele != nil; ele = ele.Next() {
		expected := q2.Dequeue().(segment)
		test := ele.Value.(segment)

		assert.Equal(t, expected.getSequenceNumber(), test.getSequenceNumber());
	}
}

func TestQueue_GetElementsEqualSequenceNumber(t *testing.T) {
	q1 := &Queue{}
	q1.Enqueue(segment{sequenceNumber: []byte{0, 0, 0, 1}})
	q1.Enqueue(segment{sequenceNumber: []byte{0, 0, 0, 4}})
	q1.Enqueue(segment{sequenceNumber: []byte{0, 0, 0, 2}})
	q1.Enqueue(segment{sequenceNumber: []byte{0, 0, 0, 5}})
	q1.Enqueue(segment{sequenceNumber: []byte{0, 0, 0, 3}})
	q1.Enqueue(segment{sequenceNumber: []byte{0, 0, 0, 6}})

	q2 := Queue{}
	q2.Enqueue(segment{sequenceNumber: []byte{0, 0, 0, 3}})

	l2 := q1.GetElementsEqualSequenceNumber(3)

	for ele := l2.Front(); ele != nil; ele = ele.Next() {
		expected := q2.Dequeue().(segment)
		test := ele.Value.(segment)

		assert.Equal(t, expected.getSequenceNumber(), test.getSequenceNumber());
	}
}

func TestQueue_RemoveElementsInRangeSequenceNumberIncluded(t *testing.T) {
	q1 := &Queue{}
	q1.Enqueue(segment{sequenceNumber: []byte{0, 0, 0, 1}})
	q1.Enqueue(segment{sequenceNumber: []byte{0, 0, 0, 4}})
	q1.Enqueue(segment{sequenceNumber: []byte{0, 0, 0, 2}})
	q1.Enqueue(segment{sequenceNumber: []byte{0, 0, 0, 5}})
	q1.Enqueue(segment{sequenceNumber: []byte{0, 0, 0, 3}})
	q1.Enqueue(segment{sequenceNumber: []byte{0, 0, 0, 6}})

	l2 := q1.RemoveElementsInRangeSequenceNumberIncluded(Position{Start: 1, End: 3})

	for ele := l2.Front(); ele != nil; ele = ele.Next() {
		test := ele.Value.(segment)

		assert.NotEqual(t, test.getSequenceNumber(), 1);
		assert.NotEqual(t, test.getSequenceNumber(), 2);
		assert.NotEqual(t, test.getSequenceNumber(), 3);
	}
}

func TestQueue_CheckType(t *testing.T) {
	q1 := &Queue{}
	q1.Enqueue(segment{sequenceNumber: []byte{0, 0, 0, 1}})

	test := func() { q1.Enqueue(1) }

	assert.Panics(t, test, "")

}