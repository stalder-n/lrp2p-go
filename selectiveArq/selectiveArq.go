package selectiveArq

import (
	. "go-protocol/container"
	. "go-protocol/lowlevel"
	"time"
)

type selectiveArq struct {
	extension Connector
	//receiver
	AckedSegment []*Segment
	AckedBitmap  *Bitmap

	//sender
	notAckedSegment         []*Segment
	notAckedBitmap          *Bitmap
	readyToSendSegmentQueue *Queue
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
	if arq.windowSize == 0 {
		arq.windowSize = 20
	}
	arq.notAckedBitmap = New(arq.windowSize)
	arq.notAckedSegment = make([]*Segment, arq.windowSize)
	arq.readyToSendSegmentQueue = NewQueue()

	arq.AckedBitmap = New(arq.windowSize)
	arq.AckedSegment = make([]*Segment, arq.windowSize)

	if arq.sequenceNumberFactory == nil {
		arq.sequenceNumberFactory = SequenceNumberFactory
	}

	arq.initialSequenceNumber = arq.sequenceNumberFactory()
	arq.currentSequenceNumber = arq.initialSequenceNumber
	arq.currentInorderNumber = 0

	if arq.extension != nil {
		return arq.extension.Open()

	}

	return nil
}

func (arq *selectiveArq) Close() error {
	return arq.extension.Close()
}

func (arq *selectiveArq) addExtension(extension Connector) {
	arq.extension = extension
}

func (arq *selectiveArq) queueTimedOutSegmentsForWrite() {
	for index, seg := range arq.notAckedSegment {
		if HasSegmentTimedOut(seg) {
			arq.readyToSendSegmentQueue.PushFront(seg)
			arq.notAckedBitmap.Remove(uint32(index))
			arq.notAckedSegment[index] = nil
		}
	}
}

func (arq *selectiveArq) writeQueuedSegments() (StatusCode, int, error) {
	sumN := 0
	for !arq.readyToSendSegmentQueue.IsEmpty() {
		seg := arq.readyToSendSegmentQueue.Dequeue().(*Segment)
		_, n, err := arq.extension.Write(seg.Buffer)
		seg.Timestamp = time.Now()

		if err != nil {
			return Fail, n, err
		}

		sumN += n
		arq.notAckedSegment[seg.GetSequenceNumber()%arq.windowSize] = seg
		arq.notAckedBitmap.Add(seg.GetSequenceNumber())
	}

	return Success, sumN, nil
}

func (arq *selectiveArq) Write(buffer []byte) (StatusCode, int, error) {
	newSegmentQueue := CreateSegments(buffer, arq.getAndIncrementCurrentSequenceNumber)

	oldWindow := arq.window
	for !newSegmentQueue.IsEmpty() && arq.window < arq.windowSize {
		ele := newSegmentQueue.Dequeue()
		arq.readyToSendSegmentQueue.Enqueue(ele)
		arq.window++
	}

	arq.queueTimedOutSegmentsForWrite()

	status, _, err := arq.writeQueuedSegments()

	if arq.window == arq.windowSize {
		return WindowFull, int(arq.window - oldWindow), nil
	}

	return status, int(arq.window - oldWindow), err
}

func (arq *selectiveArq) Read(buffer []byte) (StatusCode, int, error) {
	buf := make([]byte, SegmentMtu)
	status, n, err := arq.extension.Read(buf)
	if err != nil {
		return Fail, n, err
	}
	seg := CreateSegment(buf)

	if seg.IsFlaggedAs(FlagSelectiveACK) {
		arq.handleSelectiveAck(seg)
		copy(buffer, seg.Data)
		return AckReceived, n, err
	}

	//received data-Segment
	arq.AckedBitmap.Add(seg.GetSequenceNumber())
	arq.AckedSegment[seg.GetSequenceNumber()%arq.windowSize] = seg
	arq.writeSelectiveAck()

	//TODO handle out of order segments
	if arq.isInOrder(seg) {
		copy(buffer, seg.Data)
	}

	return status, n - seg.GetHeaderSize(), err
}

func (arq *selectiveArq) handleSelectiveAck(seg *Segment) {
	ackedSequenceNumberBitmap := New(arq.windowSize)
	ackedSequenceNumberBitmap.Init(BytesToUint32(seg.Data))

	for i := uint32(0); i < arq.windowSize; i++ {
		ele := ackedSequenceNumberBitmap.Get(i)
		if ele == 1 && arq.notAckedBitmap.Get(i) != 0 {
			seg := arq.notAckedSegment[i]
			arq.notAckedSegment[i] = nil
			arq.notAckedBitmap.Remove(seg.GetSequenceNumber())

			arq.window--
		}
	}

	for _, ele := range arq.notAckedSegment {
		if ele != nil {
			arq.readyToSendSegmentQueue.PushFront(ele)
		}
	}

	arq.writeQueuedSegments()
}

func (arq *selectiveArq) writeSelectiveAck() (StatusCode, int, error) {
	ack := CreateSelectiveAckSegment(arq.getAndIncrementCurrentSequenceNumber(), arq.AckedBitmap.ToNumber())
	return arq.extension.Write(ack.Buffer)
}

func (arq *selectiveArq) isInOrder(segment *Segment) bool {
	return true
}
