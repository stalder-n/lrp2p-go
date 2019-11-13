package selectiveArq

import (
	. "go-protocol/container"
	. "go-protocol/lowlevel"
	"time"
)

type selectiveArq struct {
	extension               Connector
	notAckedSegment         []*Segment
	readyToSendSegmentQueue Queue
	notAckedBitmap          *Bitmap
	currentInorderNumber    uint32
	initialSequenceNumber   uint32
	currentSequenceNumber   uint32
	window                  uint32
	windowSize              uint32
	sequenceNumberFactory   func() uint32
}

func (arq *selectiveArq) getAndIncrementCurrentSequenceNumber() uint32 {
	result := arq.currentSequenceNumber
	arq.currentSequenceNumber = arq.currentSequenceNumber + 1

	return result
}

func (arq *selectiveArq) Open() error {
	arq.windowSize = 20

	arq.notAckedBitmap = New(int(arq.windowSize))

	arq.readyToSendSegmentQueue = Queue{}
	arq.readyToSendSegmentQueue.New()

	arq.notAckedSegment = make([]*Segment, arq.windowSize)

	if arq.sequenceNumberFactory == nil {
		arq.sequenceNumberFactory = SequenceNumberFactory
	}

	arq.initialSequenceNumber = arq.sequenceNumberFactory()
	arq.currentSequenceNumber = arq.initialSequenceNumber
	arq.currentInorderNumber = 0

	return arq.extension.Open()
}

func (arq *selectiveArq) Close() error {
	return arq.extension.Close()
}

func (arq *selectiveArq) addExtension(extension Connector) {
	arq.extension = extension
}

func (arq *selectiveArq) parseSegments(buffer []byte) *Queue {
	result := &Queue{}
	result.New()

	var seg Segment
	currentIndex := 0
	for {
		currentIndex, seg = PeekFlaggedSegmentOfBuffer(currentIndex, arq.getAndIncrementCurrentSequenceNumber(), buffer)
		result.Enqueue(seg)
		if currentIndex == len(buffer) {
			break
		}
	}

	return result
}

func (arq *selectiveArq) queueTimedOutSegmentsForWrite() {
	for index, seg := range arq.notAckedSegment {
		if HasSegmentTimedOut(seg) {
			arq.readyToSendSegmentQueue.PushFront(seg)
			arq.notAckedBitmap.Set(uint32(index), 0)
			arq.notAckedSegment[index] = nil
		}
	}
}

func (arq *selectiveArq) writeQueuedSegments() (StatusCode, int, error) {
	sumN := 0
	for !arq.readyToSendSegmentQueue.IsEmpty() {
		seg := arq.readyToSendSegmentQueue.Dequeue().(Segment)
		_, n, err := arq.extension.Write(seg.Buffer)
		seg.Timestamp = time.Now()

		if err != nil {
			return Fail, n, err
		}

		sumN += n
		arq.notAckedSegment[seg.GetSequenceNumber()%arq.windowSize] = &seg
		arq.notAckedBitmap.Set(seg.GetSequenceNumber()%arq.windowSize, 1)
	}

	return Success, sumN, nil
}

func (arq *selectiveArq) Write(buffer []byte) (StatusCode, int, error) {
	newSegmentqueue := arq.parseSegments(buffer)

	oldwindow := arq.window
	for !newSegmentqueue.IsEmpty() && arq.window < arq.windowSize {
		ele := newSegmentqueue.Dequeue()
		arq.readyToSendSegmentQueue.Enqueue(ele)
		arq.notAckedBitmap.Set(arq.window, 1)
		arq.window++
	}

	arq.queueTimedOutSegmentsForWrite()

	status, _, err := arq.writeQueuedSegments()

	if arq.window == arq.windowSize {
		return WindowFull, int(arq.window - oldwindow), nil
	}

	return status, int(arq.window - oldwindow), err
}

func (arq *selectiveArq) writeMissingSegment() {
	for ele := range arq.notAckedSegment {
		arq.readyToSendSegmentQueue.PushFront(ele)
	}

	arq.writeQueuedSegments()
}

func (arq *selectiveArq) Read(buffer []byte) (StatusCode, int, error) {
	buf := make([]byte, SegmentMtu)
	status, n, err := arq.extension.Read(buf)
	if err != nil {
		return Fail, n, err
	}
	seg := CreateSegment(buf)

	if seg.IsFlaggedAs(FlagSelectiveACK) {
		arq.handleSelectiveAck(&seg)
		return AckReceived, n, err
	}

	//TODO Send ACK
	arq.notAckedBitmap.Set(seg.GetSequenceNumber()%arq.windowSize, 1)
	arq.notAckedSegment[seg.GetSequenceNumber()%arq.windowSize] = &seg
	arq.writeSelectiveAck()

	if arq.currentInorderNumber == 0 && len(arq.notAckedSegment) == 0 {
		arq.currentInorderNumber = seg.GetSequenceNumber();
	}

	if arq.notAckedSegment[seg.GetSequenceNumber()%arq.windowSize] != nil {
		copy(buffer, arq.notAckedSegment[seg.GetSequenceNumber()%arq.windowSize].Data)
	}

	return status, n - seg.GetHeaderSize(), err
}

func (arq *selectiveArq) handleSelectiveAck(seg *Segment) {
	ackedSegmentSequenceNumbers := New(int(arq.windowSize))
	ackedSegmentSequenceNumbers.Init(BytesToUint32(seg.Data))

	for i := uint32(0); i < arq.windowSize; i++ {
		ele := ackedSegmentSequenceNumbers.Get(i)
		if ele == 1 {
			arq.notAckedSegment[i] = nil
			arq.notAckedBitmap.Set(i, 0)
			arq.window--
		}
	}

	arq.writeMissingSegment()
}

func (arq *selectiveArq) writeSelectiveAck() (StatusCode, int, error) {
	ack := CreateSelectiveAckSegment(arq.getAndIncrementCurrentSequenceNumber(), arq.notAckedBitmap.ToNumberInverted())
	return arq.extension.Write(ack.Buffer)
}
