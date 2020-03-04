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
	sendQueue             []*segment
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
		sendQueue:                  make([]*segment, 0),
		waitingForAck:              make([]*segment, 0),
		sendSynFlag:                true,
	}
}

func (arq *selectiveArq) getAndIncrementCurrentSequenceNumber() uint32 {
	result := arq.currentSequenceNumber
	arq.currentSequenceNumber++
	return result
}

func (arq *selectiveArq) handleAck(seg *segment) {
	// TODO
}

func (arq *selectiveArq) writeAck() {
	arq.writeMutex.Lock()
	defer arq.writeMutex.Unlock()
	// TODO
}

func (arq *selectiveArq) hasAvailableSegments() bool {
	return len(arq.segmentBuffer) > 0 && arq.segmentBuffer[0].getSequenceNumber() == arq.nextExpectedSequenceNumber
}

func (arq *selectiveArq) Read(buffer []byte, timestamp time.Time) (statusCode, int, error) {
	if arq.hasAvailableSegments() {
		var seg *segment
		seg, arq.segmentBuffer = popSegment(arq.segmentBuffer)
		copy(buffer, seg.data)
		return success, len(seg.data), nil
	}
	buf := make([]byte, len(buffer))
	status, _, err := arq.extension.Read(buf, timestamp)

	if status != success {
		return status, 0, err
	}

	seg := createSegment(buf)
	if arq.nextExpectedSequenceNumber == 0 && !seg.isFlaggedAs(flagSYN) {
		return fail, 0, err
	} else if arq.nextExpectedSequenceNumber == 0 {
		arq.nextExpectedSequenceNumber = seg.getSequenceNumber()
	}
	if seg.isFlaggedAs(flagACK) {
		arq.handleAck(seg)
		return ackReceived, 0, err
	}

	if seg.getSequenceNumber() < arq.nextExpectedSequenceNumber {
		arq.writeAck()
		return invalidSegment, 0, err
	}
	insertSegmentInOrder(arq.segmentBuffer, seg)
	arq.writeAck()
	copy(buffer, seg.data)
	return success, len(seg.data), err
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
		arq.sendQueue = append(arq.sendQueue, seg)
		if currentIndex >= len(buffer) {
			break
		}
	}
}

func (arq *selectiveArq) writeQueuedSegments(timestamp time.Time) (statusCode, int, error) {
	sumN := 0
	for len(arq.sendQueue) > 0 {
		seg := arq.sendQueue[0]
		statusCode, _, err := arq.extension.Write(seg.buffer, timestamp)
		if statusCode != success {
			return statusCode, sumN, err
		}
		_, arq.sendQueue = popSegment(arq.sendQueue)
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

func insertSegmentInOrder(segments []*segment, insert *segment) []*segment {
	for i, seg := range segments {
		if insert.getSequenceNumber() < seg.getSequenceNumber() {
			segments = append(segments, nil)
			copy(segments[i+1:], segments[i:])
			segments[i] = insert
			return segments
		}
	}
	return append(segments, insert)
}

func removeSegment(segments []*segment, sequenceNumber uint32) (*segment, []*segment) {
	for i, seg := range segments {
		if seg.getSequenceNumber() == sequenceNumber {
			return seg, append(segments[:i], segments[i+1:]...)
		}
	}
	return nil, segments
}

func popSegment(segments []*segment) (*segment, []*segment) {
	return segments[0], segments[1:]
}
