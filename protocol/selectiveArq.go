package protocol

import (
	"time"
)

type selectiveArq struct {
	extension                      Connector
	notAckedSegmentQueue           Queue
	readyToSendSegmentQueue        Queue
	lastAckedSegmentSequenceNumber uint32
	lastInOrderNumber              uint32
	initialSequenceNumber          uint32
	currentSequenceNumber          uint32
	window                         uint32
	windowSize                     uint32
	sequenceNumberFactory          func() uint32
}

func (arq *selectiveArq) getAndIncrementCurrentSequenceNumber() uint32 {
	result := arq.currentSequenceNumber
	arq.currentSequenceNumber = arq.currentSequenceNumber + 1

	return result
}

func (arq *selectiveArq) Open() error {
	arq.windowSize = 20
	arq.readyToSendSegmentQueue = Queue{}
	arq.notAckedSegmentQueue = Queue{}

	if arq.sequenceNumberFactory == nil {
		arq.sequenceNumberFactory = sequenceNumberFactory
	}

	arq.initialSequenceNumber = arq.sequenceNumberFactory()
	arq.currentSequenceNumber = arq.initialSequenceNumber
	arq.lastInOrderNumber = 0
	arq.lastAckedSegmentSequenceNumber = arq.initialSequenceNumber - 1

	return arq.extension.Open()
}

func (arq *selectiveArq) Close() error {
	return arq.extension.Close()
}

func (arq *selectiveArq) addExtension(extension Connector) {
	arq.extension = extension
}

func (arq *selectiveArq) parseAndQueueSegmentsForWrite(buffer []byte) {
	currentIndex, segmentCount := 0, 0
	var seg segment
	for {
		currentIndex, seg = peekFlaggedSegmentOfBuffer(currentIndex, arq.getAndIncrementCurrentSequenceNumber(), buffer)
		arq.readyToSendSegmentQueue.Enqueue(seg)
		segmentCount++
		if currentIndex == len(buffer) {
			break
		}
	}
}

func (arq *selectiveArq) queueTimedOutSegmentsForWrite() {
	if arq.notAckedSegmentQueue.IsEmpty() {
		return
	}

	oldestSegment, ok := arq.notAckedSegmentQueue.Peek().(segment)

	if ok && hasSegmentTimedOut(&oldestSegment) {
		arq.window -= uint32(arq.notAckedSegmentQueue.Len())

		for arq.notAckedSegmentQueue.Len() != 0 {
			arq.readyToSendSegmentQueue.PushFront(arq.notAckedSegmentQueue.Dequeue())
		}
	}
}

func (arq *selectiveArq) writeQueuedSegments() (statusCode, int, error) {
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

func (arq *selectiveArq) Write(buffer []byte) (statusCode, int, error) {
	arq.parseAndQueueSegmentsForWrite(buffer)
	arq.queueTimedOutSegmentsForWrite()
	return arq.writeQueuedSegments()
}

func (arq *selectiveArq) writeAck(receivedSequenceNumber uint32) (statusCode, int, error) {
	ack := createAckSegment(arq.getAndIncrementCurrentSequenceNumber(), receivedSequenceNumber)
	return arq.extension.Write(ack.buffer)
}

func (arq *selectiveArq) hasAcksPending() bool {
	return arq.lastAckedSegmentSequenceNumber < uint32(arq.notAckedSegmentQueue.Len())
}

func (arq *selectiveArq) writeMissingSegment() {
	missingSegments := arq.notAckedSegmentQueue.GetElementGreaterSequenceNumber(arq.lastAckedSegmentSequenceNumber)

	arq.readyToSendSegmentQueue.PushFrontList(missingSegments)
	arq.writeQueuedSegments()
}

func parseSelectiveAckData(data []byte) []Position {
	var result []Position
	//truncated := bytes.Trim(data, "\x00")
	//TODO

	return result
}

func (arq *selectiveArq) Read(buffer []byte) (statusCode, int, error) {
	buf := make([]byte, segmentMtu)
	status, n, err := arq.extension.Read(buf)
	if err != nil {
		return fail, n, err
	}
	seg := createSegment(buf)

	if seg.isFlaggedAs(FlagSelectiveACK) {
		arq.handleSelectiveAck(&seg)
		return ackReceived, n, err
	}

	if arq.lastInOrderNumber == 0 && !seg.isFlaggedAs(FlagSYN) {
		return invalidSegment, n, err
	}

	//TODO

	arq.lastInOrderNumber = seg.getSequenceNumber();
	copy(buffer, seg.data)
	return status, n - seg.getHeaderSize(), err
}

func (arq *selectiveArq) writeSelectiveAck(data interface{}) (statusCode, int, error) {
	ack := createSelectiveAckSegment(arq.getAndIncrementCurrentSequenceNumber(), data)
	return arq.extension.Write(ack.buffer)

}

func (arq *selectiveArq) handleSelectiveAck(seg *segment) {
	ackedSegmentSequenceNumbers := parseSelectiveAckData(seg.data)

	for _, v := range ackedSegmentSequenceNumbers {
		ele := arq.notAckedSegmentQueue.RemoveElementsInRangeSequenceNumberIncluded(v)
		arq.window -= uint32(ele.Len())
		arq.lastAckedSegmentSequenceNumber = Max(arq.lastAckedSegmentSequenceNumber, uint32(v.End))
	}

	arq.writeMissingSegment()

}
