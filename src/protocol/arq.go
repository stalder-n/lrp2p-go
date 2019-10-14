package protocol

import (
	"math/rand"
	"sync"
)

const windowSize = 3

func initialSequenceNumber() uint32 {
	sequenceNum := rand.Uint32()
	if sequenceNum == 0 {
		sequenceNum++
	}
	return sequenceNum
}

type goBackNArqWriter struct {
	extensionDelegator
	segmentBuffer map[uint32]segment
	mutex         sync.Mutex
}

func (arq *goBackNArqWriter) init() {
	arq.segmentBuffer = map[uint32]segment{}
}

func nextSegment(currentIndex int, sequenceNum uint32, buffer []byte) (int, segment) {
	next := currentIndex + dataChunkSize
	var flag byte = 0
	if currentIndex == 0 {
		flag |= flagSyn
	}
	if next > len(buffer) {
		flag |= flagEnd
		next = len(buffer)
	}

	return next, createFlaggedSegment(sequenceNum, flag, buffer[currentIndex:next])
}

func (arq *goBackNArqWriter) Write(buffer []byte) {
	arq.mutex.Lock()
	defer arq.mutex.Unlock()

	var currentIndex = 0
	var seg segment
	var sequenceNumber = initialSequenceNumber()
	for {
		currentIndex, seg = nextSegment(currentIndex, sequenceNumber, buffer)
		sequenceNumber++
		arq.extension.Write(seg.buffer)
		arq.segmentBuffer[seg.getSequenceNumber()] = seg
		if seg.flaggedAs(flagEnd) {
			return
		}
	}
}

type goBackNArqReader struct {
	extensionDelegator
	segmentBuffer      map[uint32]segment
	nextSequenceNumber uint32
	mutex              sync.Mutex
}

func (arq *goBackNArqReader) writeAck(sequenceNumber uint32) {
	ack := createAckSegment(sequenceNumber)
	arq.extension.Write(ack.buffer)
}

func (arq *goBackNArqReader) Read() []byte {
	arq.mutex.Lock()
	defer arq.mutex.Unlock()

	seg := createSegment(arq.extension.Read())

	if !seg.flaggedAs(flagAck) {
		arq.writeAck(seg.getSequenceNumber())
	}
	return seg.data
}
