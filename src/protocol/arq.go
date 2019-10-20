package protocol

import (
	"crypto/rand"
	"sync"
)

var sequenceNumberFactory = func() uint32 {
	b := make([]byte, 4)
	_, err := rand.Read(b)
	handleError(err)
	sequenceNum := bytesToUint32(b)
	if sequenceNum == 0 {
		sequenceNum++
	}
	return sequenceNum
}

func initialSequenceNumber() uint32 {
	return sequenceNumberFactory()
}

type goBackNArq struct {
	extensionDelegator
	segmentWriteBuffer    []*segment
	segmentQueue          queue
	segmentsAcked         int
	initialSequenceNumber uint32
	lastInOrderNumber     uint32
	window                uint16
	windowSize            uint16
	writeMutex            sync.Mutex
	readMutex             sync.Mutex
}

func (arq *goBackNArq) Open() {
	arq.segmentQueue = queue{}
	if arq.windowSize == 0 {
		arq.windowSize = 10
	}
	arq.extension.Open()
}

func nextSegment(currentIndex int, sequenceNum uint32, buffer []byte) (int, segment) {
	var next = currentIndex + dataChunkSize
	var flag byte = 0
	if currentIndex == 0 {
		flag |= flagSyn
	}
	if next >= len(buffer) {
		flag |= flagEnd
		next = len(buffer)
	}
	return next, createFlaggedSegment(sequenceNum, flag, buffer[currentIndex:next])
}

func (arq *goBackNArq) queueSegments(buffer []byte) {
	var currentIndex = 0
	var segmentCount = 0
	var seg segment
	var sequenceNumber = initialSequenceNumber()
	arq.initialSequenceNumber = sequenceNumber
	arq.segmentsAcked = 0
	for {
		currentIndex, seg = nextSegment(currentIndex, sequenceNumber, buffer)
		arq.segmentQueue.Enqueue(seg)
		segmentCount++
		sequenceNumber++
		if seg.flaggedAs(flagEnd) {
			break
		}
	}
	arq.segmentWriteBuffer = make([]*segment, segmentCount)
}

func (arq *goBackNArq) Write(buffer []byte) (int, error) {
	arq.writeMutex.Lock()
	defer arq.writeMutex.Unlock()

	if len(buffer) > 0 {
		arq.queueSegments(buffer)
	}

	n, err := arq.writeQueuedSegments()
	return n, err
}

func (arq *goBackNArq) writeQueuedSegments() (int, error) {
	sumN := 0
	for !arq.segmentQueue.IsEmpty() {
		if arq.window >= arq.windowSize {
			return sumN, &windowFullError{}
		}

		seg := arq.segmentQueue.Dequeue().(segment)
		n, err := arq.extension.Write(seg.buffer)

		if err != nil {
			return sumN, err
		}
		sumN += n - headerSize
		arq.segmentWriteBuffer[seg.getSequenceNumber()-arq.initialSequenceNumber] = &seg
		arq.window++
	}

	return sumN, nil
}

func (arq *goBackNArq) writeAck(sequenceNumber uint32) (int, error) {
	ack := createAckSegment(sequenceNumber)
	return arq.extension.Write(ack.buffer)
}

func (arq *goBackNArq) Read(buffer []byte) (int, error) {
	arq.readMutex.Lock()
	defer arq.readMutex.Unlock()

	var n int
	var err error
	var seg segment

	for {
		buf := make([]byte, segmentMtu)
		n, err = arq.extension.Read(buf)
		seg = createSegment(buf)

		if seg.flaggedAs(flagAck) {
			arq.handleAck(&seg)
			break
		}

		if arq.lastInOrderNumber == 0 {
			if seg.flaggedAs(flagSyn) {
				arq.lastInOrderNumber = seg.getSequenceNumber()
			} else {
				continue
			}
		} else if seg.getSequenceNumber() != arq.lastInOrderNumber+1 {
			arq.writeAck(arq.lastInOrderNumber)
			continue
		}

		arq.writeAck(seg.getSequenceNumber())
		arq.lastInOrderNumber = seg.getSequenceNumber()
		break
	}
	copy(buffer, seg.data)
	return n, err
}

func (arq *goBackNArq) handleAck(seg *segment) {
	arq.writeMutex.Lock()
	defer arq.writeMutex.Unlock()

	sequenceNumber := seg.getSequenceNumber()
	ackedSeg := int(sequenceNumber - arq.initialSequenceNumber)
	if arq.segmentsAcked > ackedSeg {
		arq.requeueSegments(seg)
		arq.window = 0
	} else {
		arq.segmentsAcked = ackedSeg + 1
		arq.window--
	}
}

func (arq *goBackNArq) requeueSegments(ack *segment) {
	expectedSequenceNumber := ack.getExpectedSequenceNumber()
	bufferIndex := int(expectedSequenceNumber - arq.initialSequenceNumber)
	var iter int

	if arq.segmentQueue.IsEmpty() {
		iter = len(arq.segmentWriteBuffer) - 1
	} else {
		nextUnsentSegment := arq.segmentQueue.Peek().(segment)
		iter = int(nextUnsentSegment.getSequenceNumber() - arq.initialSequenceNumber - 1)
	}
	for ; iter >= bufferIndex; iter-- {
		arq.segmentQueue.PushFront(*arq.segmentWriteBuffer[iter])
	}
}

type windowFullError struct{}

func (err *windowFullError) Error() string {
	return "receive window full, can't send more segments"
}
