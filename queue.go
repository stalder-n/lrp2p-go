package goprotocol

import (
	"container/list"
	"sync"
)

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

type concurrencyQueue struct {
	queue
	mutex sync.RWMutex
}

func (q *concurrencyQueue) Enqueue(value interface{}) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.queue.Enqueue(value)
}

func (q *concurrencyQueue) PushFront(value interface{}) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.queue.PushFront(value)
}

func (q *concurrencyQueue) Dequeue() interface{} {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	return q.queue.Dequeue()
}

func (q *concurrencyQueue) Peek() interface{} {
	q.mutex.RLock()
	defer q.mutex.RUnlock()
	return q.queue.Dequeue()
}

func (q *concurrencyQueue) IsEmpty() bool {
	q.mutex.RLock()
	defer q.mutex.RUnlock()
	return q.queue.IsEmpty()
}
