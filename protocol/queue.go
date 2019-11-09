package protocol

import (
	"bytes"
	"container/list"
	"encoding/binary"
	"reflect"
)

type Queue struct {
	list        list.List
	elementType reflect.Type
}

func (q *Queue) setType(value interface{}) {
	if q.elementType == nil {
		q.elementType = reflect.TypeOf(value)
	}
}

func (q *Queue) checkType(value interface{}) {
	if reflect.TypeOf(value).Name() != q.elementType.Name() {
		panic("TypeOf value and TypeOf Queue does not match")
	}
}

func (q *Queue) Enqueue(value interface{}) {
	q.setType(value)
	q.checkType(value)
	q.list.PushBack(value)
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

//TODO make generic or refactor
func (q *Queue) GetElementGreaterSequenceNumber(sequenceNumber uint32) *list.List {
	var sequenceNumberBytes = make([]byte, 4)
	binary.BigEndian.PutUint32(sequenceNumberBytes, sequenceNumber)
	sl := list.New()

	for ele := q.list.Front(); ele != nil; ele = ele.Next() {
		seg := ele.Value.(segment)
		if bytes.Compare(seg.sequenceNumber, sequenceNumberBytes) == 1 {
			sl.PushBack(seg)
		}
	}

	return sl
}

func (q *Queue) GetElementsGreaterOrEqualsSequenceNumber(sequenceNumber uint32) *list.List {
	result := q.GetElementGreaterSequenceNumber(sequenceNumber)
	result.PushFrontList(q.GetElementsEqualSequenceNumber(sequenceNumber))
	return result
}

func (q *Queue) GetElementsSmallerSequenceNumber(sequenceNumber uint32) *list.List {
	var sequenceNumberBytes = make([]byte, 4)
	binary.BigEndian.PutUint32(sequenceNumberBytes, sequenceNumber)
	sl := list.New()

	for ele := q.list.Front(); ele != nil; ele = ele.Next() {
		seg := ele.Value.(segment)
		if bytes.Compare(seg.sequenceNumber, sequenceNumberBytes) == -1 {
			sl.PushBack(seg)
		}
	}

	return sl
}

func (q *Queue) GetElementsEqualSequenceNumber(sequenceNumber uint32) *list.List {
	var sequenceNumberBytes = make([]byte, 4)
	binary.BigEndian.PutUint32(sequenceNumberBytes, sequenceNumber)
	sl := list.New()

	for ele := q.list.Front(); ele != nil; ele = ele.Next() {
		seg := ele.Value.(segment)
		if bytes.Compare(seg.sequenceNumber, sequenceNumberBytes) == 0 {
			sl.PushBack(seg)
		}
	}

	return sl
}

func (q *Queue) RemoveElementsInRangeSequenceNumberIncluded(sequenceNumberRange Position) *list.List {
	var lowersequenceNumberBytes = make([]byte, 4)
	var highersequenceNumberBytes = make([]byte, 4)

	binary.BigEndian.PutUint32(lowersequenceNumberBytes, uint32(sequenceNumberRange.Start))
	binary.BigEndian.PutUint32(highersequenceNumberBytes, uint32(sequenceNumberRange.End))

	sl := list.New()

	for ele := q.list.Front(); ele != nil; ele = ele.Next() {
		seg := ele.Value.(segment)
		if bytes.Compare(seg.sequenceNumber, lowersequenceNumberBytes) >= 0 || bytes.Compare(seg.sequenceNumber, highersequenceNumberBytes) <= 0 {
			sl.PushBack(seg)
			q.list.Remove(ele)
		}
	}

	return sl
}
