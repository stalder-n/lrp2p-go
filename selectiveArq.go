package atp

import (
	"math"
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
// TODO slow start, congestion control
// TODO replace slices with ring buffer implementation
//		- track number of valid segments
// TODO add connection id
// TODO add version number
type selectiveArq struct {
	extension    connector
	writeMutex   sync.Mutex
	errorChannel chan error

	// receiver
	nextExpectedSequenceNumber uint32
	segmentBuffer              []*segment
	queuedForAck               []uint32
	ackThreshold               int
	receiverWindow             uint32

	// sender
	currentSequenceNumber uint32
	writeQueue            []*segment
	waitingForAck         []*segment
	congestionWindow      uint32
	sendSynFlag           bool

	// RTT
	rttToMeasure int
	granularity  float64       // clock granularity
	sRtt         float64       // smoothed round-trip time
	rttVar       float64       // round-trip time variation
	rto          time.Duration // retransmission timeout
}

const initialCongestionWindowSize = uint32(0x5)
const initialReceiverWindowSize = uint32(1<<16 - 1)
const defaultArqTimeout = 10 * time.Millisecond
const defaultAckThreshold = 5
const rttAlpha, rttBeta = 0.125, 0.25

var arqTimeout = defaultArqTimeout

func newSelectiveArq(initialSequenceNumber uint32, extension connector, errors chan error) *selectiveArq {
	extension.SetReadTimeout(arqTimeout)
	return &selectiveArq{
		extension:             extension,
		errorChannel:          errors,
		ackThreshold:          defaultAckThreshold,
		segmentBuffer:         make([]*segment, 0),
		queuedForAck:          make([]uint32, 0, defaultAckThreshold),
		currentSequenceNumber: initialSequenceNumber,
		writeQueue:            make([]*segment, 0),
		waitingForAck:         make([]*segment, 0),
		receiverWindow:        initialReceiverWindowSize,
		congestionWindow:      initialCongestionWindowSize,
		sendSynFlag:           true,
		rttToMeasure:          5,
		granularity:           float64(100 * time.Millisecond),
		rto:                   1 * time.Second,
	}
}

func (arq *selectiveArq) getAndIncrementCurrentSequenceNumber() uint32 {
	result := arq.currentSequenceNumber
	arq.currentSequenceNumber++
	return result
}

func (arq *selectiveArq) measureRTT(seg *segment, timestamp time.Time) {
	if !seg.isFlaggedAs(flagRTT) {
		return
	}
	rtt := float64(timestamp.Sub(seg.timestamp))
	if arq.sRtt == 0 {
		arq.sRtt = rtt
		arq.rttVar = arq.sRtt / 2
		arq.rto = time.Duration(arq.sRtt + math.Max(arq.granularity, 4*arq.rttVar))
	} else {
		arq.rttVar = (1-rttBeta)*arq.rttVar + rttBeta*math.Abs(arq.sRtt-rtt)
		arq.sRtt = (1-rttAlpha)*arq.sRtt + rttAlpha*rtt
		arq.rto = time.Duration(arq.sRtt + math.Max(arq.granularity, 4*arq.rttVar))
	}
}

func (arq *selectiveArq) handleAck(ack *segment, timestamp time.Time) {
	arq.writeMutex.Lock()
	defer arq.writeMutex.Unlock()

	nums := ackSegmentToSequenceNumbers(ack)
	for _, num := range nums {
		var seg *segment
		seg, arq.waitingForAck = removeSegment(arq.waitingForAck, num)
		arq.measureRTT(seg, timestamp)
	}

	var removed []*segment
	removed, arq.waitingForAck = removeAllSegmentsWhere(arq.waitingForAck, func(seg *segment) bool {
		return seg.getSequenceNumber() < nums[len(nums)-1]
	})

	for _, seg := range removed {
		arq.writeQueue = insertSegmentInOrder(arq.writeQueue, seg)
	}
	arq.increaseCongestionWindow(uint32(len(nums)-len(removed)), ack.getWindowSize())
	_, _, _ = arq.writeQueuedSegments(timestamp)
}

func (arq *selectiveArq) writeAck(addedFlags byte, timestamp time.Time) {
	arq.writeMutex.Lock()
	defer arq.writeMutex.Unlock()
	ack := createAckSegment(arq.nextExpectedSequenceNumber-1, arq.receiverWindow, arq.queuedForAck)
	ack.addFlag(addedFlags)
	arq.queuedForAck = make([]uint32, 0, arq.ackThreshold)
	ack.timestamp = timestamp
	_, _, _ = arq.extension.Write(ack.buffer, timestamp)
}

func (arq *selectiveArq) readyToACK() int {
	return len(arq.queuedForAck)
}

func (arq *selectiveArq) hasAvailableSegments() bool {
	return len(arq.segmentBuffer) > 0 && arq.segmentBuffer[0].getSequenceNumber() == arq.nextExpectedSequenceNumber
}

func (arq *selectiveArq) reduceCongestionWindow() {
	arq.congestionWindow = maxUint32(arq.congestionWindow/2, initialCongestionWindowSize)
}

func (arq *selectiveArq) increaseCongestionWindow(add, recvWindow uint32) {
	if arq.congestionWindow+add > recvWindow {
		arq.congestionWindow = recvWindow
	} else if arq.congestionWindow+add < initialCongestionWindowSize {
		arq.congestionWindow = initialCongestionWindowSize
	} else {
		arq.congestionWindow += add
	}
}

func (arq *selectiveArq) handleSuccess(receivedSeg *segment, timestamp time.Time) statusCode {
	if receivedSeg.isFlaggedAs(flagACK) {
		arq.handleAck(receivedSeg, timestamp)
		return ackReceived
	}
	if arq.nextExpectedSequenceNumber == 0 && !receivedSeg.isFlaggedAs(flagSYN) {
		return fail
	} else if arq.nextExpectedSequenceNumber == 0 {
		arq.nextExpectedSequenceNumber = receivedSeg.getSequenceNumber()
	}
	if receivedSeg.getSequenceNumber() < arq.nextExpectedSequenceNumber {
		arq.writeAck(0, timestamp)
		return invalidSegment
	}
	arq.segmentBuffer = insertSegmentInOrder(arq.segmentBuffer, receivedSeg)
	if receivedSeg.isFlaggedAs(flagRTT) {
		if arq.readyToACK() > 0 {
			arq.writeAck(0, timestamp)
		}
		arq.queuedForAck = append(arq.queuedForAck, receivedSeg.getSequenceNumber())
		arq.writeAck(flagRTT, timestamp)
	} else {
		arq.queuedForAck = append(arq.queuedForAck, receivedSeg.getSequenceNumber())
	}
	return success
}

func (arq *selectiveArq) Read(buffer []byte, timestamp time.Time) (statusCode, int, error) {
	buf := make([]byte, len(buffer))
	status, n, err := arq.extension.Read(buf, timestamp)

	switch status {
	case success:
		if s := arq.handleSuccess(createSegment(buf[:n]), timestamp); s != success {
			return s, 0, err
		}
	case timeout:
		if arq.nextExpectedSequenceNumber > 0 && arq.readyToACK() > 0 {
			arq.writeAck(0, timestamp)
		}
	default:
		return status, 0, err
	}
	var seg *segment
	if arq.hasAvailableSegments() {
		seg, arq.segmentBuffer = popSegment(arq.segmentBuffer)
		arq.nextExpectedSequenceNumber++
		if arq.readyToACK() >= arq.ackThreshold {
			arq.writeAck(0, timestamp)
		}
		copy(buffer, seg.data)
		return success, len(seg.data), err
	} else if status == success {
		if arq.readyToACK() >= arq.ackThreshold {
			arq.writeAck(0, timestamp)
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
	if arq.rttToMeasure > 0 {
		flag |= flagRTT
		arq.rttToMeasure--
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
		return timestamp.After(seg.timestamp.Add(arq.rto))
	})
	if len(removed) > 0 {
		arq.reduceCongestionWindow()
	}
	for _, seg := range removed {
		arq.writeQueue = insertSegmentInOrder(arq.writeQueue, seg)
	}
}

func (arq *selectiveArq) writeQueuedSegments(timestamp time.Time) (statusCode, int, error) {
	sumN := 0
	for len(arq.writeQueue) > 0 {
		if uint32(len(arq.waitingForAck)) >= arq.congestionWindow {
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

func maxUint32(x, y uint32) uint32 {
	if x > y {
		return x
	}
	return y
}

func maxDuration(x, y time.Duration) time.Duration {
	if x > y {
		return x
	}
	return y
}
