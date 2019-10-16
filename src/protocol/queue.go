package protocol

import "container/list"

type queue struct {
	list list.List
}

func (q *queue) Enqueue(value interface{}) {
	q.list.PushBack(value)
}

func (q *queue) PushFront(value interface{}) {
	q.list.PushFront(value)
}

func (q *queue) Dequeue() interface{} {
	if q.IsEmpty() {
		return nil
	}
	elem := q.list.Front()
	q.list.Remove(elem)
	return elem.Value
}

func (q *queue) Peek() interface{} {
	if q.IsEmpty() {
		return nil
	}
	return q.list.Front().Value
}

func (q *queue) IsEmpty() bool {
	return q.list.Len() == 0
}

type element struct {
	next  *element
	value interface{}
}
