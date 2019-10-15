package protocol

import "testing"

func TestQueue_EmptyQueue(t *testing.T) {
	q := queue{}
	if q.Dequeue() != nil {
		t.Errorf("Empty queue value not == nil")
	}
	if !q.IsEmpty() {
		t.Errorf("IsEmpty() for empty queue != true")
	}
}

func printTestError(t *testing.T, expected, actual int) {
	t.Errorf("Expected Dequeue() == %d, was %d", expected, actual)
}

func TestQueue_WithMultipleEntries(t *testing.T) {
	q := queue{}
	q.Enqueue(3)
	q.Enqueue(5)
	q.Enqueue(2)

	if q.IsEmpty() {
		t.Errorf("Queue with 3 elements shows as empty")
	}

	actual := q.Dequeue().(int)
	if actual != 3 {
		printTestError(t, 3, actual)
	}
	actual = q.Dequeue().(int)
	if actual != 5 {
		printTestError(t, 5, actual)
	}
	actual = q.Dequeue().(int)
	if actual != 2 {
		printTestError(t, 2, actual)
	}
}
