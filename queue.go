package atp

import (
	"container/list"
	"reflect"
	"sync"
)

type queue struct {
	list        *list.List
	elementType reflect.Type
}

func newQueue() *queue {
	q := &queue{}
	q.list = list.New()
	q.list.Init()

	return q
}

//TODO Conditional compilation in Golang - http://blog.ralch.com/tutorial/golang-conditional-compilation/
func (q *queue) setType(value interface{}) {
	if q.elementType == nil {
		q.elementType = reflect.TypeOf(value)
	}
}

//TODO Conditional compilation in Golang
func (q *queue) checkType(value interface{}) {
	if reflect.TypeOf(value).Name() != q.elementType.Name() {
		panic("TypeOf value and TypeOf container does not match: '" + reflect.TypeOf(value).Name() + "' '" + q.elementType.Name() + "'")
	}
}

func (q *queue) Enqueue(value interface{}) {
	q.setType(value)
	q.checkType(value)
	q.list.PushBack(value)
}

func (q *queue) EnqueueList(queue *queue) {
	q.list.PushBackList(queue.list)
}

func (q *queue) PushFront(value interface{}) {
	q.setType(value)
	q.checkType(value)

	q.list.PushFront(value)
}

func (q *queue) PushFrontList(values *list.List) {
	q.list.PushFrontList(values)
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
	return q.Len() == 0
}

func (q *queue) Len() int {
	return q.list.Len()
}

func (q *queue) SearchBy(comparator func(interface{}) bool) *list.List {
	sl := list.New()

	for ele := q.list.Front(); ele != nil; ele = ele.Next() {
		if comparator(ele.Value) {
			sl.PushBack(ele.Value)
		}
	}

	return sl
}

type concurrencyQueue struct {
	*queue
	mutex sync.RWMutex
}

func newConcurrencyQueue() *concurrencyQueue {
	queue := &concurrencyQueue{}
	queue.queue = newQueue()
	return queue
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
