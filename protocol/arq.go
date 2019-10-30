package protocol

import (
	"sync"
	"time"
)

type goBackNArq struct {
	extension             Connector
	segmentWriteBuffer    []*segment
	segmentQueue          queue
	lastSegmentAcked      int
	initialSequenceNumber uint32
	lastInOrderNumber     uint32
	window                int
	windowSize            int
	writeMutex            sync.Mutex
}

func (arq *goBackNArq) Open() error {
	arq.segmentQueue = queue{}
	if arq.windowSize == 0 {
		arq.windowSize = 10
	}
	return arq.extension.Open()
}

func (arq *goBackNArq) Close() error {
	return arq.extension.Close()
}

func (arq *goBackNArq) addExtension(extension Connector) {
	arq.extension = extension
}

func (arq *goBackNArq) queueSegments(buffer []byte) {
	var currentIndex = 0
	var segmentCount = 0
	var seg segment
	var sequenceNumber = initialSequenceNumber()
	arq.initialSequenceNumber = sequenceNumber
	arq.lastSegmentAcked = -1
	for {
		currentIndex, seg = peekFlaggedSegmentOfBuffer(currentIndex, sequenceNumber, buffer)
		arq.segmentQueue.Enqueue(seg)
		segmentCount++
		sequenceNumber++
		if seg.isFlaggedAs(FlagEND) {
			break
		}
	}
	arq.segmentWriteBuffer = make([]*segment, segmentCount)
}

func (arq *goBackNArq) acksPending() bool {
	return arq.lastSegmentAcked != -1 && arq.lastSegmentAcked < len(arq.segmentWriteBuffer)-1
}

func (arq *goBackNArq) Write(buffer []byte) (statusCode, int, error) {
	arq.writeMutex.Lock()
	defer arq.writeMutex.Unlock()

	if len(buffer) > 0 {
		if !arq.segmentQueue.IsEmpty() || arq.acksPending() {
			return pendingSegments, 0, nil
		}
		arq.queueSegments(buffer)
	}
	arq.requeueTimedOutSegments()
	status, n, err := arq.writeQueuedSegments()
	return status, n, err
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

func (arq *goBackNArq) writeQueuedSegments() (statusCode, int, error) {
	sumN := 0
	for !arq.segmentQueue.IsEmpty() {
		if arq.window >= arq.windowSize {
			return windowFull, sumN, nil
		}

		seg := arq.segmentQueue.Dequeue().(segment)
		status, n, err := arq.extension.Write(seg.buffer)
		seg.timestamp = time.Now()

		if err != nil {
			return status, sumN, err
		}
		sumN += n - seg.getHeaderSize()
		arq.segmentWriteBuffer[seg.getSequenceNumber()-arq.initialSequenceNumber] = &seg
		arq.window++
	}

	return success, sumN, nil
}

func (arq *goBackNArq) writeAck(sequenceNumber uint32) (statusCode, int, error) {
	ack := createAckSegment(sequenceNumber)
	return arq.extension.Write(ack.buffer)
}

func (arq *goBackNArq) Read(buffer []byte) (statusCode, int, error) {
	buf := make([]byte, segmentMtu)
	status, n, err := arq.extension.Read(buf)
	if err != nil {
		return fail, n, err
	}
	seg := createSegment(buf)

	if seg.isFlaggedAs(FlagACK) {
		arq.handleAck(&seg)
		return ackReceived, n, err
	}

	if arq.lastInOrderNumber == 0 {
		if !seg.isFlaggedAs(FlagSYN) {
			return invalidSegment, n, err
		}
	} else if seg.getSequenceNumber() != arq.lastInOrderNumber+1 {
		if seg.getSequenceNumber() > arq.lastInOrderNumber+1 {
			_, _, err = arq.writeAck(arq.lastInOrderNumber)
		}
		return invalidSegment, 0, err
	}

	arq.writeMutex.Lock()
	if seg.isFlaggedAs(FlagEND) {
		arq.lastInOrderNumber = 0
	} else {
		arq.lastInOrderNumber = seg.getSequenceNumber()
	}
	arq.writeMutex.Unlock()

	_, _, err = arq.writeAck(seg.getSequenceNumber())
	copy(buffer, seg.data)
	return status, n - seg.getHeaderSize(), err
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
