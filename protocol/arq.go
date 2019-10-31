package protocol

import (
	"sync"
	"time"
)

type goBackNArq struct {
	extension               Connector
	notAckedSegmentQueue    []*segment
	readyToSendSegmentQueue queue
	lastAckedSequenceNumber int //TODO fix possible overflow
	initialSequenceNumber   uint32
	lastInOrderNumber       uint32
	window                  int
	windowSize              int
	writeMutex              sync.Mutex
}

func (arq *goBackNArq) Open() error {
	arq.readyToSendSegmentQueue = queue{}
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

func (arq *goBackNArq) setupSegmentsQueues(buffer []byte) {
	var currentIndex = 0
	var segmentCount = 0
	var seg segment
	var sequenceNumber = initialSequenceNumber()
	arq.initialSequenceNumber = sequenceNumber
	arq.lastAckedSequenceNumber = -1
	for {
		currentIndex, seg = peekFlaggedSegmentOfBuffer(currentIndex, sequenceNumber, buffer)
		arq.readyToSendSegmentQueue.Enqueue(seg)
		segmentCount++
		sequenceNumber++
		if seg.isFlaggedAs(FlagEND) {
			break
		}
	}
	arq.notAckedSegmentQueue = make([]*segment, segmentCount)
}

func (arq *goBackNArq) hasAcksPending() bool {
	return arq.lastAckedSequenceNumber != -1 && arq.lastAckedSequenceNumber < len(arq.notAckedSegmentQueue)-1
}

func (arq *goBackNArq) Write(buffer []byte) (statusCode, int, error) {

	if buffer == nil {
		panic("Paramater is nil")
	}

	arq.writeMutex.Lock()

	if len(buffer) > 0 {
		if !arq.readyToSendSegmentQueue.IsEmpty() || arq.hasAcksPending() {
			return pendingSegments, 0, nil
		}
		arq.setupSegmentsQueues(buffer)
	}

	arq.writeMutex.Unlock()
	return arq.WriteTimedOutSegments()
}

func (arq *goBackNArq) WriteTimedOutSegments() (statusCode, int, error) {
	arq.writeMutex.Lock()
	defer arq.writeMutex.Unlock()

	if arq.lastAckedSequenceNumber+1 < len(arq.notAckedSegmentQueue) {
		pendingSeg := arq.notAckedSegmentQueue[arq.lastAckedSequenceNumber+1]
		if pendingSeg != nil && hasSegmentTimedOut(pendingSeg) {
			arq.requeueSegments()
		}
	}

	status, n, err := arq.writeQueuedSegments()
	return status, n, err
}

func (arq *goBackNArq) writeQueuedSegments() (statusCode, int, error) {
	sumN := 0
	for !arq.readyToSendSegmentQueue.IsEmpty() {
		if arq.window >= arq.windowSize {
			return windowFull, sumN, nil
		}

		seg := arq.readyToSendSegmentQueue.Dequeue().(segment)
		status, n, err := arq.extension.Write(seg.buffer)
		seg.timestamp = time.Now()

		if err != nil {
			return status, sumN, err
		}
		sumN += n - seg.getHeaderSize()
		arq.notAckedSegmentQueue[seg.getSequenceNumber()-arq.initialSequenceNumber] = &seg
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

	if arq.lastInOrderNumber == 0 && !seg.isFlaggedAs(FlagSYN) {
		return invalidSegment, n, err
	}

	if seg.getSequenceNumber() != arq.lastInOrderNumber+1 {
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
	if arq.lastAckedSequenceNumber == ackedSeg {
		arq.requeueSegments()
		arq.window = 0
	} else if arq.lastAckedSequenceNumber < ackedSeg {
		arq.window -= ackedSeg - arq.lastAckedSequenceNumber
		arq.lastAckedSequenceNumber = ackedSeg
	}
}

func (arq *goBackNArq) requeueSegments() {
	for i := len(arq.notAckedSegmentQueue) - 1; i > arq.lastAckedSequenceNumber; i-- {
		if arq.notAckedSegmentQueue[i] != nil {
			arq.readyToSendSegmentQueue.PushFront(*arq.notAckedSegmentQueue[i])
		}
	}
}
