package protocol

import (
	"time"
)

type goBackNArq struct {
	extension                      Connector
	notAckedSegmentQueue           queue
	readyToSendSegmentQueue        queue
	lastAckedSegmentSequenceNumber uint32
	lastInOrderNumber              uint32
	initialSequenceNumber          uint32
	currentSequenceNumber          uint32
	window                         uint32
	windowSize                     uint32
	sequenceNumberFactory          func() uint32
}

func (arq *goBackNArq) GetAndIncrementCurrentSequenceNumber() uint32 {
	result := arq.currentSequenceNumber
	arq.currentSequenceNumber = arq.currentSequenceNumber + 1

	return result
}

func (arq *goBackNArq) Open() error {
	arq.windowSize = 20
	arq.readyToSendSegmentQueue = queue{}
	arq.notAckedSegmentQueue = queue{}

	if arq.sequenceNumberFactory == nil {
		arq.sequenceNumberFactory = sequenceNumberFactory
	}

	arq.initialSequenceNumber = arq.sequenceNumberFactory()
	arq.currentSequenceNumber = arq.initialSequenceNumber
	arq.lastInOrderNumber = 0
	arq.lastAckedSegmentSequenceNumber = arq.initialSequenceNumber - 1

	return arq.extension.Open()
}

func (arq *goBackNArq) Close() error {
	return arq.extension.Close()
}

func (arq *goBackNArq) AddExtension(extension Connector) {
	arq.extension = extension
}

func (arq *goBackNArq) parseByteStreamAndQueueSegmentsForWrite(buffer []byte) {
	currentIndex, segmentCount := 0, 0
	var seg segment
	for {
		currentIndex, seg = peekFlaggedSegmentOfBuffer(currentIndex, arq.GetAndIncrementCurrentSequenceNumber(), buffer)
		arq.readyToSendSegmentQueue.Enqueue(seg)
		segmentCount++
		if seg.isFlaggedAs(FlagEND) {
			break
		}
	}
}

func (arq *goBackNArq) Write(buffer []byte) (statusCode, int, error) {

	if buffer == nil {
		panic("Parameter is nil")
	}

	arq.parseByteStreamAndQueueSegmentsForWrite(buffer)
	arq.queueTimedOutSegmentsForWrite()
	return arq.writeQueuedSegments()
}

func (arq *goBackNArq) queueTimedOutSegmentsForWrite() {
	if arq.notAckedSegmentQueue.IsEmpty() {
		return
	}

	oldestSegment, ok := arq.notAckedSegmentQueue.Peek().(segment)

	if ok && hasSegmentTimedOut(&oldestSegment) {
		for arq.notAckedSegmentQueue.Len() != 0 {
			arq.readyToSendSegmentQueue.PushFront(arq.notAckedSegmentQueue.Dequeue())
		}

		arq.window = 0
	}
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
		arq.notAckedSegmentQueue.Enqueue(seg)
		arq.window++
	}

	return success, sumN, nil
}

func (arq *goBackNArq) writeAck(receivedSequenceNumber uint32) (statusCode, int, error) {
	ack := createAckSegment(arq.GetAndIncrementCurrentSequenceNumber(), receivedSequenceNumber)
	return arq.extension.Write(ack.buffer)
}

func (arq *goBackNArq) hasAcksPending() bool {
	return arq.lastAckedSegmentSequenceNumber < uint32(arq.notAckedSegmentQueue.Len())
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

	if arq.lastInOrderNumber != 0 && seg.getSequenceNumber() > arq.lastInOrderNumber+1 {
		_, _, err = arq.writeAck(arq.lastInOrderNumber)
		return invalidSegment, 0, err
	}

	if arq.lastInOrderNumber != 0 && seg.getSequenceNumber() != arq.lastInOrderNumber+1 {
		return invalidSegment, 0, err
	}

	if seg.isFlaggedAs(FlagEND) {
		arq.lastInOrderNumber = 0
	} else {
		arq.lastInOrderNumber = seg.getSequenceNumber()
	}

	_, _, err = arq.writeAck(seg.getSequenceNumber())
	copy(buffer, seg.data)
	return status, n - seg.getHeaderSize(), err
}

func (arq *goBackNArq) handleAck(seg *segment) {
	ackedSegmentSequenceNumber := bytesToUint32(seg.data)
	if arq.notAckedSegmentQueue.Len() != 0 && arq.lastAckedSegmentSequenceNumber == ackedSegmentSequenceNumber {
		arq.writeMissingSegment()
	} else if arq.lastAckedSegmentSequenceNumber < ackedSegmentSequenceNumber {
		arq.window -= ackedSegmentSequenceNumber - arq.lastAckedSegmentSequenceNumber
		arq.lastAckedSegmentSequenceNumber = ackedSegmentSequenceNumber
	}
}

func (arq *goBackNArq) writeMissingSegment() {
	missingSegments := arq.notAckedSegmentQueue.GetElementGreaterSequenceNumber(arq.lastAckedSegmentSequenceNumber + 1)

	arq.readyToSendSegmentQueue.PushFrontList(missingSegments)
	arq.writeQueuedSegments()
}
