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

type goBackNArqExtension struct {
	extensionDelegator
	segmentWriteBuffer map[uint32]segment
	nextSequenceNumber uint32
	writeMutex         sync.Mutex
	readMutex          sync.Mutex
}

func (arq *goBackNArqExtension) init() {
	arq.segmentWriteBuffer = map[uint32]segment{}
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

func (arq *goBackNArqExtension) Write(buffer []byte) {
	arq.writeMutex.Lock()
	defer arq.writeMutex.Unlock()

	var currentIndex = 0
	var seg segment
	var sequenceNumber = initialSequenceNumber()
	for {
		currentIndex, seg = nextSegment(currentIndex, sequenceNumber, buffer)
		sequenceNumber++
		arq.extension.Write(seg.buffer)
		arq.segmentWriteBuffer[seg.getSequenceNumber()] = seg
		if seg.flaggedAs(flagEnd) {
			return
		}
	}
}

func (arq *goBackNArqExtension) writeAck(sequenceNumber uint32) {
	ack := createAckSegment(sequenceNumber)
	arq.extension.Write(ack.buffer)
}

func (arq *goBackNArqExtension) Read() []byte {
	arq.readMutex.Lock()
	defer arq.readMutex.Unlock()

	seg := createSegment(arq.extension.Read())

	if !seg.flaggedAs(flagAck) {
		arq.writeAck(seg.getSequenceNumber())
	}
	return seg.data
}
