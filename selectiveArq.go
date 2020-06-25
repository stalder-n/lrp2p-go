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
	receiverWindow             uint32

	// sender
	currentSequenceNumber uint32
	writeQueue            []*segment
	waitingForAck         []*segment
	sendSynFlag           bool

	// CUBIC
	cwnd                float64
	wMax                float64
	ssthresh            float64
	aggressiveness      float64
	lastCongestionEvent time.Time
	lastCongestionType  congestionType

	// RTT
	rttToMeasure int
	granularity  float64       // clock granularity
	sRtt         float64       // smoothed round-trip time
	rttVar       float64       // round-trip time variation
	rto          time.Duration // retransmission timeout
}

const initialReceiverWindowSize = uint32(1<<16 - 1)
const defaultArqTimeout = 1 * time.Millisecond
const rttAlpha, rttBeta = 0.125, 0.25
const betaCubic float64 = 0.7
const defaultAggressiveness = 1

type congestionType int

const (
	noCongestion congestionType = iota
	segmentLoss
	segmentTimeout
)

func newSelectiveArq(extension connector, errors chan error) *selectiveArq {
	extension.SetReadTimeout(defaultArqTimeout)
	return &selectiveArq{
		extension:           extension,
		errorChannel:        errors,
		segmentBuffer:       make([]*segment, 0),
		writeQueue:          make([]*segment, 0),
		waitingForAck:       make([]*segment, 0),
		receiverWindow:      initialReceiverWindowSize,
		sendSynFlag:         true,
		cwnd:                1,
		aggressiveness:      defaultAggressiveness,
		ssthresh:            float64(initialReceiverWindowSize / 10),
		lastCongestionEvent: time.Now(),
		rttToMeasure:        5,
		granularity:         float64(100 * time.Millisecond),
		rto:                 1 * time.Second,
	}
}

func (arq *selectiveArq) getAndIncrementCurrentSequenceNumber() uint32 {
	result := arq.currentSequenceNumber
	arq.currentSequenceNumber++
	return result
}

func (arq *selectiveArq) measureRTT(seg *segment, timestamp time.Time) {
	if arq.rttToMeasure <= 0 {
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
	arq.rttToMeasure--
}

func (arq *selectiveArq) handleAck(ack *segment, timestamp time.Time) {
	arq.writeMutex.Lock()
	defer arq.writeMutex.Unlock()

	lastInOrder := ack.getSequenceNumber()
	ackedSequence := bytesToUint32(ack.data[:4])
	var ackedSeg *segment
	ackedSeg, arq.waitingForAck = removeSegment(arq.waitingForAck, ackedSequence)
	if ackedSeg == nil || ackedSequence < lastInOrder {
		return
	}
	arq.measureRTT(ackedSeg, timestamp)

	congType := noCongestion
	if len(arq.waitingForAck) > 0 && (ackedSequence-lastInOrder) >= arq.waitingForAck[0].retransmitThresh {
		var retransmit *segment
		retransmit, arq.waitingForAck = arq.waitingForAck[0], arq.waitingForAck[1:]
		retransmit.retransmitThresh += defaultRetransmitThresh
		arq.writeQueue = insertSegmentInOrder(arq.writeQueue, retransmit)
		_, _, _ = arq.writeQueuedSegments(timestamp)
		congType = segmentLoss
	}
	arq.computeCongestionWindow(congType)
}

func (arq *selectiveArq) writeAck(seg *segment, timestamp time.Time) {
	arq.writeMutex.Lock()
	defer arq.writeMutex.Unlock()
	lastInOrder := arq.nextExpectedSequenceNumber - 1
	if arq.nextExpectedSequenceNumber == 0 {
		lastInOrder = 0
	}
	ack := createAckSegment(lastInOrder, seg.getSequenceNumber(), arq.receiverWindow)
	ack.timestamp = timestamp
	_, _, _ = arq.extension.Write(ack.buffer, timestamp)
}

func (arq *selectiveArq) hasAvailableSegments() bool {
	return len(arq.segmentBuffer) > 0 && arq.segmentBuffer[0].getSequenceNumber() == arq.nextExpectedSequenceNumber
}

func (arq *selectiveArq) computeCongestionWindow(t congestionType) {
	mult := betaCubic
	switch t {
	case noCongestion:
		if arq.cwnd < arq.ssthresh {
			arq.cwnd++
		} else {
			t := float64(time.Now().Sub(arq.lastCongestionEvent))
			wEst := arq.estimateTCPWindow(t)
			wCubic := arq.cwnd + (arq.computeWCubic(t+arq.sRtt)-arq.cwnd)/arq.cwnd
			arq.cwnd = math.Max(wEst, wCubic)
		}
	case segmentTimeout:
		mult = 0.5
		fallthrough
	case segmentLoss:
		arq.wMax = arq.cwnd
		arq.ssthresh = arq.cwnd * betaCubic
		arq.ssthresh = math.Max(arq.ssthresh, 2)
		arq.cwnd = math.Max(1, arq.cwnd*mult)

	}
	arq.lastCongestionType = t
}

func (arq *selectiveArq) computeWCubic(t float64) float64 {
	seconds := t / float64(time.Second)
	var K float64
	if arq.lastCongestionType == segmentTimeout {
		K = 0
	} else {
		K = arq.computeK()
	}
	return arq.aggressiveness*math.Pow(seconds-K, 3) + arq.wMax
}

func (arq *selectiveArq) computeK() float64 {
	return math.Pow(arq.wMax*(1-betaCubic)/arq.aggressiveness, 1.0/3.0)
}

func (arq *selectiveArq) estimateTCPWindow(t float64) float64 {
	seconds := t / float64(time.Second)
	rttSeconds := arq.sRtt / float64(time.Second)
	return arq.wMax*betaCubic + 3*(1-betaCubic)/(1+betaCubic) + (seconds / rttSeconds)
}

func (arq *selectiveArq) handleSuccess(receivedSeg *segment, timestamp time.Time) statusCode {
	if receivedSeg.isFlaggedAs(flagACK) {
		arq.handleAck(receivedSeg, timestamp)
		return ackReceived
	}
	if receivedSeg.getSequenceNumber() >= arq.nextExpectedSequenceNumber {
		arq.segmentBuffer = insertSegmentInOrder(arq.segmentBuffer, receivedSeg)
	}
	arq.writeAck(receivedSeg, timestamp)
	return success
}

func (arq *selectiveArq) Read(buffer []byte, timestamp time.Time) (statusCode, int, error) {
	buf := make([]byte, len(buffer))
	status, n, err := arq.extension.Read(buf, timestamp)

	if status == success {
		status = arq.handleSuccess(createSegment(buf[:n]), timestamp)
	}

	var seg *segment
	if arq.hasAvailableSegments() {
		seg, arq.segmentBuffer = popSegment(arq.segmentBuffer)
		arq.nextExpectedSequenceNumber++
		copy(buffer, seg.data)
		return success, len(seg.data), err
	} else if status == success {
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
		return timestamp.After(seg.timestamp.Add(arq.rto))
	})
	if len(removed) > 0 {
		arq.computeCongestionWindow(segmentTimeout)
	}
	for _, seg := range removed {
		arq.writeQueue = insertSegmentInOrder(arq.writeQueue, seg)
	}
}

func (arq *selectiveArq) writeQueuedSegments(timestamp time.Time) (statusCode, int, error) {
	sumN := 0
	for len(arq.writeQueue) > 0 {
		if len(arq.waitingForAck) >= int(arq.cwnd) {
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
