package atp

import (
	"sync"
	"time"
)

//   ┌───────┐          ┌───────┐
//   │Socket1│          │Socket2│
//   └───┬───┘          └───┬───┘
//       │   Segment(1)     │
//       │─────────────────>│
//       │                  │
//       │   Segment(2)     │
//       │<─────────────────│
//       │                  │
//       │     ACK(1)       │
//       │<─ ─ ─ ─ ─ ─ ─ ─ ─│
//       │                  │
//       │     ACK(2)       │
//       │ ─ ─ ─ ─ ─ ─ ─ ─ >│
//   ┌───┴───┐          ┌───┴───┐
//   │Socket1│          │Socket2│
//   └───────┘          └───────┘
// The sequence above shows the default case for this ARQ component
// TODO: slow start, congestion control
// TODO Replace slices with more efficient solution (e.g. ring buffer)
type selectiveArq struct {
	extension    connector
	writeMutex   sync.Mutex
	errorChannel chan error

	// receiver
	nextExpectedSequenceNumber uint32
	segmentBuffer              []*segment
	queuedForAck               []uint32
	segsSinceLastAck           int
	ackThreshold               int

	// sender
	currentSequenceNumber uint32
	writeQueue            []*segment
	waitingForAck         []*segment
	windowSize            int
	sendSynFlag           bool
}

const defaultArqTimeout = 10 * time.Millisecond
const defaultInitialWindowSize = 20

var arqTimeout = defaultArqTimeout

func newSelectiveArq(initialSequenceNumber uint32, extension connector, errors chan error) *selectiveArq {
	extension.SetReadTimeout(arqTimeout)
	return &selectiveArq{
		extension:                  extension,
		errorChannel:               errors,
		nextExpectedSequenceNumber: 0,
		ackThreshold:               5,
		segmentBuffer:              make([]*segment, 0),
		queuedForAck:               make([]uint32, 0),
		currentSequenceNumber:      initialSequenceNumber,
		writeQueue:                 make([]*segment, 0),
		waitingForAck:              make([]*segment, 0),
		windowSize:                 defaultInitialWindowSize,
		sendSynFlag:                true,
	}
}

func (arq *selectiveArq) getAndIncrementCurrentSequenceNumber() uint32 {
	result := arq.currentSequenceNumber
	arq.currentSequenceNumber++
	return result
}

func (arq *selectiveArq) handleAck(ack *segment, timestamp time.Time) {
	arq.writeMutex.Lock()
	defer arq.writeMutex.Unlock()

	nums := ackSegmentToSequenceNumbers(ack)
	for _, num := range nums {
		_, arq.waitingForAck = removeSegment(arq.waitingForAck, num)
	}

	var removed []*segment
	removed, arq.waitingForAck = removeAllSegmentsWhere(arq.waitingForAck, func(seg *segment) bool {
		return seg.getSequenceNumber() < nums[len(nums)-1]
	})

	for _, seg := range removed {
		arq.writeQueue = insertSegmentInOrder(arq.writeQueue, seg)
	}
	_, _, _ = arq.writeQueuedSegments(timestamp)
}

func (arq *selectiveArq) writeAck(timestamp time.Time) {
	arq.writeMutex.Lock()
	defer arq.writeMutex.Unlock()
	ack := createAckSegment(arq.nextExpectedSequenceNumber-1, arq.queuedForAck)
	arq.queuedForAck = make([]uint32, arq.ackThreshold)
	ack.timestamp = timestamp
	_, _, _ = arq.extension.Write(ack.buffer, timestamp)
	arq.segsSinceLastAck = 0
}

func (arq *selectiveArq) hasAvailableSegments() bool {
	return len(arq.segmentBuffer) > 0 && arq.segmentBuffer[0].getSequenceNumber() == arq.nextExpectedSequenceNumber
}

func (arq *selectiveArq) Read(buffer []byte, timestamp time.Time) (statusCode, int, error) {
	buf := make([]byte, len(buffer))
	status, n, err := arq.extension.Read(buf, timestamp)

	switch status {
	case success:
		receivedSeg := createSegment(buf[:n])
		if receivedSeg.isFlaggedAs(flagACK) {
			arq.handleAck(receivedSeg, timestamp)
			return ackReceived, 0, err
		}
		if arq.nextExpectedSequenceNumber == 0 && !receivedSeg.isFlaggedAs(flagSYN) {
			return fail, 0, err
		} else if arq.nextExpectedSequenceNumber == 0 {
			arq.nextExpectedSequenceNumber = receivedSeg.getSequenceNumber()
		}
		if receivedSeg.getSequenceNumber() < arq.nextExpectedSequenceNumber {
			arq.writeAck(timestamp)
			return invalidSegment, 0, err
		}
		arq.segmentBuffer = insertSegmentInOrder(arq.segmentBuffer, receivedSeg)
		arq.queuedForAck = insertUin32InOrder(arq.queuedForAck, receivedSeg.getSequenceNumber())
		arq.segsSinceLastAck++
	case timeout:
		if arq.nextExpectedSequenceNumber > 0 && arq.segsSinceLastAck > 0 {
			arq.writeAck(timestamp)
		}
	default:
		return status, 0, err
	}
	var seg *segment
	if arq.hasAvailableSegments() {
		seg, arq.segmentBuffer = popSegment(arq.segmentBuffer)
		arq.nextExpectedSequenceNumber++
		if arq.segsSinceLastAck >= arq.ackThreshold {
			arq.writeAck(timestamp)
		}
		copy(buffer, seg.data)
		return success, len(seg.data), err
	} else if status == success {
		if arq.segsSinceLastAck >= arq.ackThreshold {
			arq.writeAck(timestamp)
		}
		return invalidSegment, 0, err
	}
	return status, 0, err
}

func (arq *selectiveArq) getNextSegmentInBuffer(currentIndex int, sequenceNum uint32, buffer []byte) (int, *segment) {
	var next = currentIndex + getDataChunkSize()
	var flag byte = 0
	if arq.sendSynFlag {
		flag |= flagSYN
		arq.sendSynFlag = false
	}
	if next >= len(buffer) {
		next = len(buffer)
	}
	return next, createFlaggedSegment(sequenceNum, flag, buffer[currentIndex:next])
}

func (arq *selectiveArq) queueNewSegments(buffer []byte) {
	var seg *segment
	currentIndex := 0
	for {
		currentIndex, seg = arq.getNextSegmentInBuffer(currentIndex, arq.getAndIncrementCurrentSequenceNumber(), buffer)
		arq.writeQueue = append(arq.writeQueue, seg)
		if currentIndex >= len(buffer) {
			break
		}
	}
}

func (arq *selectiveArq) requeueTimedOutSegments(timestamp time.Time) {
	removed := make([]*segment, 0, len(arq.waitingForAck))
	removed, arq.waitingForAck = removeAllSegmentsWhere(arq.waitingForAck, func(seg *segment) bool {
		return seg.hasTimedOut(timestamp)
	})
	for _, seg := range removed {
		arq.writeQueue = insertSegmentInOrder(arq.writeQueue, seg)
	}
}

func (arq *selectiveArq) writeQueuedSegments(timestamp time.Time) (statusCode, int, error) {
	sumN := 0
	for len(arq.writeQueue) > 0 {
		if len(arq.waitingForAck) >= arq.windowSize {
			return windowFull, sumN, nil
		}
		seg := arq.writeQueue[0]
		statusCode, _, err := arq.extension.Write(seg.buffer, timestamp)
		seg.timestamp = timestamp
		if statusCode != success {
			return statusCode, sumN, err
		}
		_, arq.writeQueue = popSegment(arq.writeQueue)
		arq.waitingForAck = insertSegmentInOrder(arq.waitingForAck, seg)
		sumN += len(seg.data)
	}
	return success, sumN, nil
}

func (arq *selectiveArq) Write(buffer []byte, timestamp time.Time) (statusCode, int, error) {
	arq.writeMutex.Lock()
	defer arq.writeMutex.Unlock()

	arq.requeueTimedOutSegments(timestamp)
	if len(buffer) > 0 {
		arq.queueNewSegments(buffer)
	}
	return arq.writeQueuedSegments(timestamp)
}

func (arq *selectiveArq) Close() error {
	return arq.extension.Close()
}

func (arq *selectiveArq) SetReadTimeout(timeout time.Duration) {
	arq.extension.SetReadTimeout(timeout)
}

func (arq *selectiveArq) ConnectTo(remoteHost string, remotePort int) {
	arq.extension.ConnectTo(remoteHost, remotePort)
}

func (arq *selectiveArq) reportError(err error) {
	if err != nil {
		arq.errorChannel <- err
	}
}
