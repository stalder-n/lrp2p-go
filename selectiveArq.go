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
type selectiveArq struct {
	extension    connector
	writeMutex   sync.Mutex
	errorChannel chan error

	// receiver
	nextExpectedSequenceNumber uint32
	segmentBuffer              []*segment

	// sender
	currentSequenceNumber uint32
	writeQueue            []*segment
	waitingForAck         []*segment
	sendSynFlag           bool
}

func newSelectiveArq(initialSequenceNumber uint32, extension connector, errors chan error) *selectiveArq {
	return &selectiveArq{
		extension:                  extension,
		errorChannel:               errors,
		nextExpectedSequenceNumber: 0,
		segmentBuffer:              make([]*segment, 0),
		currentSequenceNumber:      initialSequenceNumber,
		writeQueue:                 make([]*segment, 0),
		waitingForAck:              make([]*segment, 0),
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
	ack := createAckSegment(arq.nextExpectedSequenceNumber-1, arq.segmentBuffer)
	ack.timestamp = timestamp
	_, _, _ = arq.extension.Write(ack.buffer, timestamp)
}

func (arq *selectiveArq) hasAvailableSegments() bool {
	return len(arq.segmentBuffer) > 0 && arq.segmentBuffer[0].getSequenceNumber() == arq.nextExpectedSequenceNumber
}

func (arq *selectiveArq) Read(buffer []byte, timestamp time.Time) (statusCode, int, error) {
	if arq.hasAvailableSegments() {
		var seg *segment
		seg, arq.segmentBuffer = popSegment(arq.segmentBuffer)
		arq.nextExpectedSequenceNumber++
		copy(buffer, seg.data)
		return success, len(seg.data), nil
	}
	buf := make([]byte, len(buffer))
	status, n, err := arq.extension.Read(buf, timestamp)

	if status != success {
		return status, 0, err
	}

	seg := createSegment(buf)
	if seg.isFlaggedAs(flagACK) {
		arq.handleAck(seg, timestamp)
		return ackReceived, 0, err
	}
	
	if arq.nextExpectedSequenceNumber == 0 && !seg.isFlaggedAs(flagSYN) {
		return fail, 0, err
	} else if arq.nextExpectedSequenceNumber == 0 {
		arq.nextExpectedSequenceNumber = seg.getSequenceNumber()
	}

	if seg.getSequenceNumber() < arq.nextExpectedSequenceNumber {
		arq.writeAck(timestamp)
		return invalidSegment, 0, err
	} else if seg.getSequenceNumber() > arq.nextExpectedSequenceNumber {
		insertSegmentInOrder(arq.segmentBuffer, seg)
		arq.writeAck(timestamp)
		return invalidSegment, 0, err
	} else {
		arq.nextExpectedSequenceNumber++
	}
	arq.writeAck(timestamp)
	copy(buffer, seg.data)
	return success, n - headerLength, err
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
	currentIndex := 0
	for {
		currentIndex, seg := getNextSegmentInBuffer(currentIndex, arq.getAndIncrementCurrentSequenceNumber(), buffer)
		arq.writeQueue = append(arq.writeQueue, seg)
		if currentIndex >= len(buffer) {
			break
		}
	}
}

func (arq *selectiveArq) writeQueuedSegments(timestamp time.Time) (statusCode, int, error) {
	sumN := 0
	for len(arq.writeQueue) > 0 {
		seg := arq.writeQueue[0]
		statusCode, _, err := arq.extension.Write(seg.buffer, timestamp)
		seg.timestamp = timestamp
		if statusCode != success {
			return statusCode, sumN, err
		}
		_, arq.writeQueue = popSegment(arq.writeQueue)
		insertSegmentInOrder(arq.waitingForAck, seg)
		sumN += len(seg.data)
	}
	return success, sumN, nil
}

func (arq *selectiveArq) Write(buffer []byte, timestamp time.Time) (statusCode, int, error) {
	arq.writeMutex.Lock()
	defer arq.writeMutex.Unlock()

	arq.queueNewSegments(buffer)
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
