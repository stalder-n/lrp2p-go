package goprotocol

import (
	"time"
)

type goBackNArq struct {
	extension                      Connector
	notAckedSegmentQueue           *Queue
	readyToSendSegmentQueue        *Queue
	lastAckedSegmentSequenceNumber uint32
	lastInOrderNumber              uint32
	initialSequenceNumber          uint32
	currentSequenceNumber          uint32
	window                         uint32
	windowSize                     uint32
	sequenceNumberFactory          func() uint32
}

func newGoBackNArq(extension Connector) *goBackNArq {
	arq := &goBackNArq{
		extension:               extension,
		notAckedSegmentQueue:    &Queue{},
		readyToSendSegmentQueue: &Queue{},
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
	arq.readyToSendSegmentQueue = NewQueue()
	arq.notAckedSegmentQueue = NewQueue()

	if arq.sequenceNumberFactory == nil {
		arq.sequenceNumberFactory = SequenceNumberFactory
	}

	arq.initialSequenceNumber = arq.sequenceNumberFactory()
	arq.currentSequenceNumber = arq.initialSequenceNumber
	arq.lastInOrderNumber = arq.initialSequenceNumber - 1
	arq.lastAckedSegmentSequenceNumber = arq.initialSequenceNumber - 1

	return arq.extension.Open()
}

func (arq *goBackNArq) Close() error {
	return arq.extension.Close()
}

func (arq *goBackNArq) addExtension(extension Connector) {
	arq.extension = extension
}

func (arq *goBackNArq) parseAndQueueSegmentsForWrite(buffer []byte) {
	segs := CreateSegments(buffer, arq.getAndIncrementCurrentSequenceNumber)
	arq.readyToSendSegmentQueue.EnqueueList(segs)
}

func (arq *goBackNArq) queueTimedOutSegmentsForWrite() {
	if arq.notAckedSegmentQueue.IsEmpty() {
		return
	}

	oldestSegment, ok := arq.notAckedSegmentQueue.Peek().(Segment)

	if ok && HasSegmentTimedOut(&oldestSegment) {
		arq.window -= uint32(arq.notAckedSegmentQueue.Len())

		for arq.notAckedSegmentQueue.Len() != 0 {
			arq.readyToSendSegmentQueue.PushFront(arq.notAckedSegmentQueue.Dequeue())
		}
	}
}

func (arq *goBackNArq) writeQueuedSegments(timestamp time.Time) (StatusCode, int, error) {
	sumN := 0
	for !arq.readyToSendSegmentQueue.IsEmpty() {
		if arq.window >= arq.windowSize {
			return WindowFull, sumN, nil
		}

		seg := arq.readyToSendSegmentQueue.Dequeue().(*Segment)
		status, n, err := arq.extension.Write(seg.Buffer, timestamp)
		seg.Timestamp = time.Now()

		if err != nil {
			return status, sumN, err
		}
		sumN += n - seg.GetHeaderSize()
		arq.notAckedSegmentQueue.Enqueue(seg)
		arq.window++
	}

	return Success, sumN, nil
}

func (arq *goBackNArq) Write(buffer []byte, timestamp time.Time) (StatusCode, int, error) {
	//TODO consolidate window size handling
	arq.parseAndQueueSegmentsForWrite(buffer)
	arq.queueTimedOutSegmentsForWrite()
	return arq.writeQueuedSegments(timestamp)
}

func (arq *goBackNArq) writeAck(receivedSequenceNumber uint32, timestamp time.Time) (StatusCode, int, error) {
	ack := CreateAckSegment(arq.getAndIncrementCurrentSequenceNumber(), receivedSequenceNumber)
	return arq.extension.Write(ack.Buffer, timestamp)
}

func (arq *goBackNArq) hasAcksPending() bool {
	return arq.lastAckedSegmentSequenceNumber < uint32(arq.notAckedSegmentQueue.Len())
}

func (arq *goBackNArq) Read(buffer []byte, timestamp time.Time) (StatusCode, int, error) {
	buf := make([]byte, SegmentMtu)
	status, n, err := arq.extension.Read(buf, timestamp)
	if err != nil {
		return Fail, n, err
	}
	seg := CreateSegment(buf)

	if seg.IsFlaggedAs(FlagACK) {
		arq.handleAck(seg, timestamp)
		return AckReceived, n, err
	}

	if arq.lastInOrderNumber == 0 && !seg.IsFlaggedAs(FlagSYN) {
		return InvalidSegment, n, err
	}

	if arq.lastInOrderNumber != 0 && seg.GetSequenceNumber() > arq.lastInOrderNumber+1 {
		_, _, err = arq.writeAck(arq.lastInOrderNumber, timestamp)
		return InvalidSegment, 0, err
	}

	if arq.lastInOrderNumber != 0 && seg.GetSequenceNumber() != arq.lastInOrderNumber+1 {
		return InvalidSegment, 0, err
	}

	arq.lastInOrderNumber = seg.GetSequenceNumber()
	_, _, err = arq.writeAck(seg.GetSequenceNumber(), timestamp)

	copy(buffer, seg.Data)
	return status, n - seg.GetHeaderSize(), err
}

func (arq *goBackNArq) handleAck(seg *Segment, timestamp time.Time) {
	ackedSegmentSequenceNumber := BytesToUint32(seg.Data)
	if arq.notAckedSegmentQueue.Len() != 0 && arq.lastAckedSegmentSequenceNumber == ackedSegmentSequenceNumber {
		arq.writeMissingSegment(timestamp)
	} else if arq.lastAckedSegmentSequenceNumber < ackedSegmentSequenceNumber {
		arq.window -= ackedSegmentSequenceNumber - arq.lastAckedSegmentSequenceNumber
		arq.lastAckedSegmentSequenceNumber = ackedSegmentSequenceNumber
	}
}

func (arq *goBackNArq) writeMissingSegment(timestamp time.Time) {
	missingSegments := arq.notAckedSegmentQueue.SearchBy(areElementsGreaterSequenceNumber(arq.lastAckedSegmentSequenceNumber))

	arq.readyToSendSegmentQueue.PushFrontList(missingSegments)
	arq.writeQueuedSegments(timestamp)
}

func (arq *goBackNArq) SetReadTimeout(t time.Duration) {
	arq.extension.SetReadTimeout(t)
}

func areElementsGreaterSequenceNumber(sequenceNumber uint32) func(seg interface{}) bool {
	return func(seg interface{}) bool {
		if seg.(*Segment).GetSequenceNumber() > sequenceNumber {
			return true
		} else {
			return false
		}
	}
}
