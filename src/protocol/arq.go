package protocol

import (
	"crypto/rand"
	"sync"
	"time"
)

var retransmissionTimeout = 200 * time.Millisecond

var sequenceNumberFactory = func() uint32 {
	b := make([]byte, 4)
	_, err := rand.Read(b)
	handleError(err)
	sequenceNum := bytesToUint32(b)
	if sequenceNum == 0 {
		sequenceNum++
	}
	return sequenceNum
}

func initialSequenceNumber() uint32 {
	return sequenceNumberFactory()
}

func hasSegmentTimedOut(seg *segment) bool {
	timeout := seg.timestamp.Add(retransmissionTimeout)
	return time.Now().After(timeout)
}

type goBackNArq struct {
	extensionDelegator
	segmentWriteBuffer    []*segment
	segmentQueue          queue
	lastSegmentAcked      int
	initialSequenceNumber uint32
	lastInOrderNumber     uint32
	window                int
	windowSize            int
	writeMutex            sync.Mutex
	readMutex             sync.Mutex
}

func (arq *goBackNArq) Open() {
	arq.segmentQueue = queue{}
	if arq.windowSize == 0 {
		arq.windowSize = 10
	}
	arq.extension.Open()
}

func nextSegment(currentIndex int, sequenceNum uint32, buffer []byte) (int, segment) {
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

func (arq *goBackNArq) queueSegments(buffer []byte) {
	var currentIndex = 0
	var segmentCount = 0
	var seg segment
	var sequenceNumber = initialSequenceNumber()
	arq.initialSequenceNumber = sequenceNumber
	arq.lastSegmentAcked = -1
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

func (arq *goBackNArq) acksPending() bool {
	return arq.lastSegmentAcked != -1 && arq.lastSegmentAcked < len(arq.segmentWriteBuffer)-1
}

func (arq *goBackNArq) Write(buffer []byte) (int, error) {
	arq.writeMutex.Lock()
	defer arq.writeMutex.Unlock()

	if len(buffer) > 0 {
		if !arq.segmentQueue.IsEmpty() || arq.acksPending() {
			return 0, &pendingSegmentsError{}
		}
		arq.queueSegments(buffer)
	}
	arq.requeueTimedOutSegments()
	n, err := arq.writeQueuedSegments()
	return n, err
}

func (arq *goBackNArq) requeueTimedOutSegments() {
	if arq.lastSegmentAcked+1 >= len(arq.segmentWriteBuffer) {
		return
	}
	pendingSeg := arq.segmentWriteBuffer[arq.lastSegmentAcked+1]
	if pendingSeg != nil && hasSegmentTimedOut(pendingSeg) {
		arq.requeueSegments()
	}
}

func (arq *goBackNArq) writeQueuedSegments() (int, error) {
	sumN := 0
	for !arq.segmentQueue.IsEmpty() {
		if arq.window >= arq.windowSize {
			return sumN, &windowFullError{}
		}

		seg := arq.segmentQueue.Dequeue().(segment)
		n, err := arq.extension.Write(seg.buffer)
		seg.timestamp = time.Now()

		if err != nil {
			return sumN, err
		}
		sumN += n - headerSize
		arq.segmentWriteBuffer[seg.getSequenceNumber()-arq.initialSequenceNumber] = &seg
		arq.window++
	}

	return sumN, nil
}

func (arq *goBackNArq) writeAck(sequenceNumber uint32) (int, error) {
	ack := createAckSegment(sequenceNumber)
	return arq.extension.Write(ack.buffer)
}

func (arq *goBackNArq) Read(buffer []byte) (int, error) {
	arq.readMutex.Lock()
	defer arq.readMutex.Unlock()

	var n int
	var err error
	var seg segment

	buf := make([]byte, segmentMtu)
	n, err = arq.extension.Read(buf)
	seg = createSegment(buf)

	if seg.flaggedAs(flagAck) {
		arq.handleAck(&seg)
		return 0, &ackReceivedError{}
	}

	if arq.lastInOrderNumber == 0 {
		if !seg.flaggedAs(flagSyn) {
			return 0, &invalidSegmentError{}
		}
	} else if seg.getSequenceNumber() != arq.lastInOrderNumber+1 {
		if seg.getSequenceNumber() > arq.lastInOrderNumber+1 {
			arq.writeAck(arq.lastInOrderNumber)
		}
		return 0, &invalidSegmentError{}
	}

	if seg.flaggedAs(flagEnd) {
		arq.lastInOrderNumber = 0
	} else {
		arq.lastInOrderNumber = seg.getSequenceNumber()
	}
	arq.writeAck(seg.getSequenceNumber())
	copy(buffer, seg.data)
	return n, err
}

func (arq *goBackNArq) handleAck(seg *segment) {
	arq.writeMutex.Lock()
	defer arq.writeMutex.Unlock()

	sequenceNumber := seg.getSequenceNumber()
	ackedSeg := int(sequenceNumber - arq.initialSequenceNumber)
	if arq.lastSegmentAcked == ackedSeg {
		arq.requeueSegments()
		arq.window = 0
	} else if arq.lastSegmentAcked < ackedSeg {
		arq.window -= ackedSeg - arq.lastSegmentAcked
		arq.lastSegmentAcked = ackedSeg
	}
}

func (arq *goBackNArq) requeueSegments() {
	for i := len(arq.segmentWriteBuffer) - 1; i > arq.lastSegmentAcked; i-- {
		if arq.segmentWriteBuffer[i] != nil {
			arq.segmentQueue.PushFront(*arq.segmentWriteBuffer[i])
		}
	}
}

type windowFullError struct{}

func (err *windowFullError) Error() string {
	return "receive window full, can't send more segments"
}

type ackReceivedError struct{}

func (err *ackReceivedError) Error() string {
	return "no data received, read ACK segment"
}

type invalidSegmentError struct{}

func (err *invalidSegmentError) Error() string {
	return "received out-of-order segment"
}

type pendingSegmentsError struct{}

func (err *pendingSegmentsError) Error() string {
	return "not all segments from previous write were sent/ACKed"
}
