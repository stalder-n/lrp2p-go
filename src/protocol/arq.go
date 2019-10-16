package protocol

import (
	"math/rand"
	"sync"
)

const windowSize uint32 = 3

func initialSequenceNumber() uint32 {
	sequenceNum := rand.Uint32()
	if sequenceNum == 0 {
		sequenceNum++
	}
	return sequenceNum
}

type goBackNArqExtension struct {
	extensionDelegator
	segmentWriteBuffer    []*segment
	segmentQueue          queue
	initialSequenceNumber uint32
	lastInOrderNumber     uint32
	window                uint32
	writeMutex            sync.Mutex
	windowLock            sync.Cond
	dataChannel           chan *segment
}

func (arq *goBackNArqExtension) Open() {
	arq.segmentQueue = queue{}
	arq.extension.Open()
	arq.windowLock = sync.Cond{
		L: &sync.Mutex{},
	}
	arq.dataChannel = make(chan *segment)
	go arq.read()
}

func (arq *goBackNArqExtension) read() {
	for {
		seg := createSegment(arq.extension.Read())
		if seg.flaggedAs(flagAck) {
			arq.handleAck(seg)
		} else {
			arq.dataChannel <- seg
		}
	}
}

func (arq *goBackNArqExtension) handleAck(seg *segment) {
	arq.windowLock.L.Lock()
	defer arq.windowLock.L.Unlock()
	defer arq.windowLock.Signal()

	sequenceNumber := seg.getSequenceNumber()
	ackedSeg := arq.segmentWriteBuffer[sequenceNumber-arq.initialSequenceNumber]
	if !ackedSeg.flaggedAs(flagAcked) {
		addFlags(ackedSeg.buffer, flagAcked)
		if arq.window > 0 {
			arq.window--
		}
	} else {
		arq.requeueSegments(seg)
		arq.window = 0
	}

}

func (arq *goBackNArqExtension) requeueSegments(ack *segment) {
	expectedSequenceNumber := ack.getExpectedSequenceNumber()
	bufferIndex := int(expectedSequenceNumber - arq.initialSequenceNumber)
	for i := len(arq.segmentWriteBuffer) - 1; i >= bufferIndex; i-- {
		arq.segmentQueue.PushFront(arq.segmentWriteBuffer[i])
	}
}

func nextSegment(currentIndex int, sequenceNum uint32, buffer []byte) (int, *segment) {
	var next = currentIndex + dataChunkSize
	var flag byte = 0
	if currentIndex == 0 {
		flag |= flagSyn
	}
	if next >= len(buffer) {
		flag |= flagEnd
		next = len(buffer)
	}
	return next, createFlaggedSegment(sequenceNum, flag, buffer[currentIndex:next])
}

func (arq *goBackNArqExtension) queueSegments(buffer []byte) {
	var currentIndex = 0
	var seg *segment
	var segmentCount = 0
	var sequenceNumber = initialSequenceNumber()
	arq.initialSequenceNumber = sequenceNumber
	for {
		currentIndex, seg = nextSegment(currentIndex, sequenceNumber, buffer)
		arq.segmentQueue.Enqueue(seg)
		segmentCount++
		sequenceNumber++
		if seg.flaggedAs(flagEnd) {
			break
		}
	}
	arq.segmentWriteBuffer = make([]*segment, segmentCount)
}

func (arq *goBackNArqExtension) allAcked() bool {
	for _, seg := range arq.segmentWriteBuffer {
		if seg == nil || !seg.flaggedAs(flagAcked) {
			return false
		}
	}
	return true
}

func (arq *goBackNArqExtension) Write(buffer []byte) {
	arq.writeMutex.Lock()
	defer arq.writeMutex.Unlock()

	arq.queueSegments(buffer)
	for {
		arq.writeQueuedSegments()
		arq.windowLock.L.Lock()
		if arq.allAcked() {
			arq.windowLock.L.Unlock()
			return
		} else {
			arq.windowLock.Wait()
			arq.windowLock.L.Unlock()
		}

	}
}

func (arq *goBackNArqExtension) writeQueuedSegments() {
	for !arq.segmentQueue.IsEmpty() {
		arq.windowLock.L.Lock()
		for arq.window >= windowSize {
			arq.windowLock.Wait()
		}

		seg := arq.segmentQueue.Dequeue().(*segment)
		arq.extension.Write(seg.buffer)
		arq.segmentWriteBuffer[seg.getSequenceNumber()-arq.initialSequenceNumber] = seg
		arq.window++
		arq.windowLock.L.Unlock()
	}
}

func (arq *goBackNArqExtension) writeAck(sequenceNumber uint32) {
	ack := createAckSegment(sequenceNumber)
	arq.extension.Write(ack.buffer)
}

func (arq *goBackNArqExtension) IsExpectedSegment(seg *segment) bool {
	return seg.getSequenceNumber() == arq.lastInOrderNumber+1
}

func (arq *goBackNArqExtension) Read() []byte {
	seg := <-arq.dataChannel
	sequenceNumber := seg.getSequenceNumber()
	if arq.lastInOrderNumber == 0 {
		if !seg.flaggedAs(flagSyn) {
			return arq.Read()
		}
	} else {
		if !arq.IsExpectedSegment(seg) {
			arq.writeAck(arq.lastInOrderNumber)
			return arq.Read()
		}
	}
	if seg.flaggedAs(flagEnd) {
		arq.lastInOrderNumber = 0
	} else {
		arq.lastInOrderNumber = sequenceNumber
	}
	arq.writeAck(sequenceNumber)
	return seg.data
}
