package atp

import (
	"time"
)

type goBackNArq struct {
	extension               Connector
	notAckedSegmentQueue    *queue
	readyToSendSegmentQueue *queue
	lastInOrderNumber       uint32
	initialSequenceNumber   uint32
	currentSequenceNumber   uint32
	window                  uint32
	windowSize              uint32
	sequenceNumberFactory   func() uint32
}

func newGoBackNArq(extension Connector) *goBackNArq {
	arq := &goBackNArq{
		extension:               extension,
		notAckedSegmentQueue:    &queue{},
		readyToSendSegmentQueue: &queue{},
		windowSize:              10,
	}
	return arq
}

func (arq *goBackNArq) getAndIncrementCurrentSequenceNumber() uint32 {
	result := arq.currentSequenceNumber
	arq.currentSequenceNumber = arq.currentSequenceNumber + 1

	return result
}

func (arq *goBackNArq) Open() error {
	arq.windowSize = 20
	arq.readyToSendSegmentQueue = newQueue()
	arq.notAckedSegmentQueue = newQueue()

	if arq.sequenceNumberFactory == nil {
		arq.sequenceNumberFactory = generateRandomSequenceNumber
	}

	arq.initialSequenceNumber = arq.sequenceNumberFactory()
	arq.currentSequenceNumber = arq.initialSequenceNumber
	arq.lastInOrderNumber = arq.initialSequenceNumber - 1
	arq.lastInOrderNumber = arq.initialSequenceNumber - 1

	return arq.extension.Open()
}

func (arq *goBackNArq) Close() error {
	return arq.extension.Close()
}

func (arq *goBackNArq) addExtension(extension Connector) {
	arq.extension = extension
}

func (arq *goBackNArq) parseAndQueueSegmentsForWrite(buffer []byte) {
	segs := createSegments(buffer, arq.getAndIncrementCurrentSequenceNumber)
	arq.readyToSendSegmentQueue.EnqueueList(segs)
}

func (arq *goBackNArq) queueTimedOutSegmentsForWrite(time time.Time) {
	if arq.notAckedSegmentQueue.IsEmpty() {
		return
	}

	oldestSegment, ok := arq.notAckedSegmentQueue.Peek().(segment)

	if ok && hasSegmentTimedOut(&oldestSegment, time) {
		arq.window -= uint32(arq.notAckedSegmentQueue.Len())

		for arq.notAckedSegmentQueue.Len() != 0 {
			arq.readyToSendSegmentQueue.PushFront(arq.notAckedSegmentQueue.Dequeue())
		}
	}
}

func (arq *goBackNArq) writeQueuedSegments(timestamp time.Time) (statusCode, int, error) {
	sumN := 0
	for !arq.readyToSendSegmentQueue.IsEmpty() {
		if arq.window >= arq.windowSize {
			return windowFull, sumN, nil
		}

		seg := arq.readyToSendSegmentQueue.Dequeue().(*segment)
		status, n, err := arq.extension.Write(seg.buffer, timestamp)
		seg.timestamp = time.Now()

		if err != nil {
			return status, sumN, err
		}
		sumN += n - seg.getHeaderSize()
		arq.notAckedSegmentQueue.Enqueue(seg)
		arq.window++
	}

	return success, sumN, nil
}

func (arq *goBackNArq) Write(buffer []byte, timestamp time.Time) (statusCode, int, error) {
	//TODO consolidate window size handling
	arq.parseAndQueueSegmentsForWrite(buffer)
	arq.queueTimedOutSegmentsForWrite(timestamp)
	return arq.writeQueuedSegments(timestamp)
}

func (arq *goBackNArq) writeAck(receivedSequenceNumber uint32, timestamp time.Time) (statusCode, int, error) {
	ack := createAckSegment(arq.getAndIncrementCurrentSequenceNumber(), receivedSequenceNumber)
	return arq.extension.Write(ack.buffer, timestamp)
}

func (arq *goBackNArq) hasAcksPending() bool {
	return arq.lastInOrderNumber < uint32(arq.notAckedSegmentQueue.Len())
}

func (arq *goBackNArq) Read(buffer []byte, timestamp time.Time) (statusCode, int, error) {
	buf := make([]byte, segmentMtu)
	status, n, err := arq.extension.Read(buf, timestamp)
	if err != nil {
		return fail, n, err
	}
	seg := createSegment(buf)

	if seg.isFlaggedAs(flagACK) {
		arq.handleAck(seg, timestamp)
		return ackReceived, n, err
	}

	if arq.lastInOrderNumber == 0 && !seg.isFlaggedAs(flagSYN) {
		return invalidSegment, n, err
	}

	if arq.lastInOrderNumber != 0 && seg.getSequenceNumber() > arq.lastInOrderNumber+1 {
		_, _, err = arq.writeAck(arq.lastInOrderNumber, timestamp)
		return invalidSegment, 0, err
	}

	if arq.lastInOrderNumber != 0 && seg.getSequenceNumber() != arq.lastInOrderNumber+1 {
		return invalidSegment, 0, err
	}

	arq.lastInOrderNumber = seg.getSequenceNumber()
	_, _, err = arq.writeAck(seg.getSequenceNumber(), timestamp)

	copy(buffer, seg.data)
	return status, n - seg.getHeaderSize(), err
}

func (arq *goBackNArq) handleAck(seg *segment, timestamp time.Time) {
	ackedSegmentSequenceNumber := bytesToUint32(seg.data)
	if arq.notAckedSegmentQueue.Len() != 0 && arq.lastInOrderNumber == ackedSegmentSequenceNumber {
		arq.writeMissingSegment(timestamp)
	} else if arq.lastInOrderNumber < ackedSegmentSequenceNumber {
		arq.window -= ackedSegmentSequenceNumber - arq.lastInOrderNumber
		arq.lastInOrderNumber = ackedSegmentSequenceNumber
	}
}

func (arq *goBackNArq) writeMissingSegment(timestamp time.Time) {
	missingSegments := arq.notAckedSegmentQueue.SearchBy(areElementsGreaterSequenceNumber(arq.lastInOrderNumber))

	arq.readyToSendSegmentQueue.PushFrontList(missingSegments)
	_, _, err := arq.writeQueuedSegments(timestamp)
	handleError(err)
}

func (arq *goBackNArq) SetReadTimeout(t time.Duration) {
	arq.extension.SetReadTimeout(t)
}

func areElementsGreaterSequenceNumber(sequenceNumber uint32) func(seg interface{}) bool {
	return func(seg interface{}) bool {
		if seg.(*segment).getSequenceNumber() > sequenceNumber {
			return true
		} else {
			return false
		}
	}
}
