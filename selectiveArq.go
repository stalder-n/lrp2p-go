package atp

import (
	"container/list"
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
// TODO add version number
type selectiveArq struct {
	writeMutex   sync.Mutex
	write        func([]byte) (statusCode, int, error)
	errorChannel chan error

	// receiver
	nextExpectedSequenceNumber uint32
	segmentBuffer              *ringBufferRcv
	receiverWindow             uint32

	// sender
	currentSequenceNumber uint32
	writeQueue            *segmentQueue
	waitingForAck         *ringBufferSnd
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
const rttAlpha, rttBeta = 0.125, 0.25
const betaCubic float64 = 0.7
const defaultAggressiveness = 1

type congestionType int

const (
	noCongestion congestionType = iota
	segmentLoss
	segmentTimeout
)

func newSelectiveArq(writeAction func([]byte) (statusCode, int, error), errors chan error) *selectiveArq {
	return &selectiveArq{
		write:               writeAction,
		errorChannel:        errors,
		segmentBuffer:       NewRingBufferRcv(initialReceiverWindowSize),
		writeQueue:          &segmentQueue{l: list.New()},
		waitingForAck:       NewRingBufferSnd(initialReceiverWindowSize),
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
	ackedSeg, _, _ := arq.waitingForAck.remove(ackedSequence)
	if ackedSeg == nil || ackedSequence < lastInOrder {
		return
	}
	arq.measureRTT(ackedSeg, timestamp)

	congType := noCongestion
	if arq.waitingForAck.numOfSegments() > 0 && (ackedSequence-lastInOrder) >= arq.waitingForAck.first().retransmitThresh {
		var retransmit *segment
		retransmit = arq.waitingForAck.first()
		retransmit.addFlag(flagRTO)
		retransmit.retransmitThresh += defaultRetransmitThresh
		status, _, _ := arq.write(retransmit.buffer)
		retransmit.updateTimestamp(status, timestamp)
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
	status, _, _ := arq.write(ack.buffer)
	ack.updateTimestamp(status, timestamp)
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

func (arq *selectiveArq) processReceivedSegment(buffer []byte, removeMultiple bool, timestamp time.Time) (statusCode, []*segment) {
	if len(buffer) > 0 {
		seg := createSegment(buffer)
		if seg.isFlaggedAs(flagACK) {
			arq.handleAck(seg, timestamp)
			return ackReceived, nil
		}
		if seg.getSequenceNumber() >= arq.nextExpectedSequenceNumber {
			arq.segmentBuffer.insert(seg)
		}
		arq.writeAck(seg, timestamp)
	}

	segs := arq.segmentBuffer.removeSequence(removeMultiple)
	if len(segs) > 0 {
		arq.nextExpectedSequenceNumber = segs[len(segs)-1].getSequenceNumber() + 1
		return success, segs
	}
	return invalidSegment, nil
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
	arq.writeMutex.Lock()
	defer arq.writeMutex.Unlock()

	var seg *segment
	currentIndex := 0
	for {
		currentIndex, seg = arq.getNextSegmentInBuffer(currentIndex, arq.getAndIncrementCurrentSequenceNumber(), buffer)
		arq.writeQueue.push(seg)
		if currentIndex >= len(buffer) {
			break
		}
	}
}

func (arq *selectiveArq) retransmitTimedOutSegments(timestamp time.Time) {
	arq.writeMutex.Lock()
	defer arq.writeMutex.Unlock()

	removed := arq.waitingForAck.getTimedout(timestamp, arq.rto)
	if len(removed) > 0 {
		arq.computeCongestionWindow(segmentTimeout)
	}
	for _, seg := range removed {
		seg.addFlag(flagRTO)
		status, _, _ := arq.write(seg.buffer)
		seg.updateTimestamp(status, timestamp)
	}
}

func (arq *selectiveArq) writeQueuedSegments(timestamp time.Time) (statusCode, error) {
	arq.writeMutex.Lock()
	defer arq.writeMutex.Unlock()

	for arq.writeQueue.size() > 0 {
		if arq.waitingForAck.numOfSegments() >= uint32(arq.cwnd) {
			return windowFull, nil
		}
		seg := arq.writeQueue.peek()
		status, _, err := arq.write(seg.buffer)
		seg.updateTimestamp(status, timestamp)
		if status != success {
			return status, err
		}
		inserted, err := arq.waitingForAck.insertSequence(seg)
		if inserted {
			arq.writeQueue.pop()
		} else if err == nil {
			_, arq.waitingForAck = arq.waitingForAck.resize(arq.waitingForAck.size() + initialReceiverWindowSize)
			inserted, err = arq.waitingForAck.insertSequence(seg)
			arq.writeQueue.pop()
		}
	}
	return success, nil
}

func (arq *selectiveArq) reportError(err error) {
	if err != nil {
		arq.errorChannel <- err
	}
}

type segmentQueue struct {
	l *list.List
}

func (queue *segmentQueue) push(seg *segment) {
	queue.l.PushBack(seg)
}

func (queue *segmentQueue) peek() *segment {
	return queue.l.Front().Value.(*segment)
}

func (queue *segmentQueue) pop() *segment {
	return queue.l.Remove(queue.l.Front()).(*segment)
}

func (queue *segmentQueue) size() int {
	return queue.l.Len()
}
