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

func printTestError(t *testing.T, name string, expected, actual int) {
	t.Errorf("Expected %s() == %d, was %d", name, expected, actual)
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
	q := queue{}
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
	q := queue{}
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
