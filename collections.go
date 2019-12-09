package atp

import (
	"container/list"
	"reflect"
	"sync"
)

type bitmap struct {
	bitmapData     []uint32
	data           []*segment
	sequenceNumber uint32
	mutex          sync.Mutex
}

func newEmptyBitmap(size uint32) *bitmap {
	bmap := &bitmap{}
	bmap.bitmapData = make([]uint32, size)
	bmap.data = make([]*segment, size)
	return bmap
}

func newBitmap(size, sequenceNumber, initialBitmap uint32) *bitmap {
	bmap := newEmptyBitmap(size)
	bmap.sequenceNumber = sequenceNumber
	bmap.init(initialBitmap)
	return bmap
}

func (m *bitmap) init(bmap uint32) {
	for i := 0; bmap != 0; i++ {
		var mod uint32
		bmap, mod = bitmapDiff(bmap)
		m.bitmapData[i] = mod
	}
}

func bitmapDiff(bitmap uint32) (uint32, uint32) {
	return bitmap / 2, bitmap % 2
}

func (m *bitmap) Add(sequenceNumber uint32, data *segment) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if sequenceNumber < m.sequenceNumber {
		return
	}
	if m.sequenceNumber == 0 {
		m.sequenceNumber = sequenceNumber
	}

	index := sequenceNumber - m.sequenceNumber
	m.bitmapData[index] = 1
	m.data[index] = data
}

func (m *bitmap) Slide() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	for m.bitmapData[0] == 1 {
		m.slideOne()
	}
}

func (m *bitmap) slideOne() {
	m.sequenceNumber++
	copy(m.bitmapData, m.bitmapData[1:])
	copy(m.data, m.data[1:])
	m.bitmapData[len(m.bitmapData)-1] = 0
}

func (m *bitmap) ToNumber() uint32 {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	var result uint32
	for i := 0; i < len(m.bitmapData); i++ {
		bit := m.bitmapData[i]
		result |= bit << i
	}

	return result
}

func (m *bitmap) GetAndRemoveInorder() *segment {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.bitmapData[0] == 1 {
		seg := m.data[0]
		m.slideOne()
		return seg
	}
	return nil
}

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

func (q *queue) PushFrontQueue(values *queue) {
	q.list.PushFrontList(values.list)
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
