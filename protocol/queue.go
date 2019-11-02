package protocol

import (
	"bytes"
	"container/list"
	"encoding/binary"
	"reflect"
)

type queue struct {
	list        list.List
	elementType reflect.Type
	once        bool
}

func (q *queue) setType(value interface{}) {
	if !q.once {
		q.elementType = reflect.TypeOf(value)
		q.once = true
	}
}

func (q *queue) checkType(value interface{}) {
	if reflect.TypeOf(value) != q.elementType {
		panic("TypeOf value and TypeOf queue does not match")
	}
}

func (q *queue) Enqueue(value interface{}) {
	q.setType(value)
	q.checkType(value)
	q.list.PushBack(value)
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

func (q *queue) GetElementGreaterSequenceNumber(sequenceNumber uint32) *list.List {
	var sequenceNumberBytes = make([]byte, 4)
	binary.BigEndian.PutUint32(sequenceNumberBytes, sequenceNumber)
	sl := list.New()

	for ele := q.list.Front(); ele != nil; ele = ele.Next() {
		seg := ele.Value.(segment)
		if bytes.Compare(seg.sequenceNumber, sequenceNumberBytes) == 0 || bytes.Compare(seg.sequenceNumber, sequenceNumberBytes) == 1 {
			sl.PushBack(seg)
		}
	}

	return sl
}
