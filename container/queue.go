package container

import (
	"container/list"
	"reflect"
)

type Queue struct {
	list        *list.List
	elementType reflect.Type
}

func NewQueue() *Queue {
	q := &Queue{}
	q.list = &list.List{}
	q.list.Init()

	return q
}

func (q *Queue) setType(value interface{}) {
	if q.elementType == nil {
		q.elementType = reflect.TypeOf(value)
	}
}

func (q *Queue) checkType(value interface{}) {
	if reflect.TypeOf(value).Name() != q.elementType.Name() {
		panic("TypeOf value and TypeOf container does not match: '" + reflect.TypeOf(value).Name() + "' '" + q.elementType.Name() + "'")
	}
}

func (q *Queue) Enqueue(value interface{}) {
	q.setType(value)
	q.checkType(value)
	q.list.PushBack(value)
}

func (q *Queue) EnqueueList(queue *Queue) {
	q.list.PushBackList(queue.list)
}

func (q *Queue) PushFront(value interface{}) {
	q.setType(value)
	q.checkType(value)

	q.list.PushFront(value)
}

func (q *Queue) PushFrontList(values *list.List) {
	q.list.PushFrontList(values)
}

func (q *Queue) Dequeue() interface{} {
	if q.IsEmpty() {
		return nil
	}
	elem := q.list.Front()
	q.list.Remove(elem)
	return elem.Value
}

func (q *Queue) Peek() interface{} {
	if q.IsEmpty() {
		return nil
	}
	return q.list.Front().Value
}

func (q *Queue) IsEmpty() bool {
	return q.Len() == 0
}

func (q *Queue) Len() int {
	return q.list.Len()
}

func (q *Queue) SearchBy(comparator func(interface{}) bool) *list.List {
	sl := list.New()

	for ele := q.list.Front(); ele != nil; ele = ele.Next() {
		if comparator(ele.Value) {
			sl.PushBack(ele.Value)
		}
	}

	return sl
}
