package goprotocol

import (
	"time"
)

type selectiveArq struct {
	extension Connector
	//receiver
	AckedBitmap *Bitmap

	//sender
	notAckedSegment         []*Segment
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
	arq.notAckedSegment = make([]*Segment, arq.windowSize)
	arq.readyToSendSegmentQueue = NewQueue()
	arq.AckedBitmap = NewBitmap(arq.windowSize)

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
			arq.notAckedSegment[index] = nil
		}
	}
}

func (arq *selectiveArq) writeQueuedSegments(timestamp time.Time) (StatusCode, int, error) {
	sumN := 0
	for !arq.readyToSendSegmentQueue.IsEmpty() {
		seg := arq.readyToSendSegmentQueue.Dequeue().(*Segment)
		_, n, err := arq.extension.Write(seg.Buffer, timestamp)
		seg.Timestamp = time.Now()

		if err != nil {
			return Fail, n, err
		}

		sumN += n
		arq.notAckedSegment[seg.GetSequenceNumber()%arq.windowSize] = seg
	}

	return Success, sumN, nil
}

func (arq *selectiveArq) Write(buffer []byte, timestamp time.Time) (StatusCode, int, error) {
	newSegmentQueue := CreateSegments(buffer, arq.getAndIncrementCurrentSequenceNumber)

	oldWindow := arq.window
	for !newSegmentQueue.IsEmpty() && arq.window < arq.windowSize {
		ele := newSegmentQueue.Dequeue()
		arq.readyToSendSegmentQueue.Enqueue(ele)
		arq.window++
	}

	arq.queueTimedOutSegmentsForWrite()

	status, _, err := arq.writeQueuedSegments(timestamp)

	if arq.window == arq.windowSize {
		return WindowFull, int(arq.window - oldWindow), nil
	}

	return status, int(arq.window - oldWindow), err
}

func (arq *selectiveArq) Read(buffer []byte, timestamp time.Time) (StatusCode, int, error) {
	buf := make([]byte, SegmentMtu)
	status, n, err := arq.extension.Read(buf, timestamp)
	if err != nil {
		return Fail, n, err
	}
	seg := CreateSegment(buf)

	if seg.IsFlaggedAs(FlagSelectiveACK) {
		arq.handleSelectiveAck(seg, timestamp)
		copy(buffer, seg.Data)
		return AckReceived, n, err
	}

	//received data-Segment:
	arq.AckedBitmap.Add(seg.GetSequenceNumber(), seg)

	segOrdered := arq.AckedBitmap.GetAndRemoveInorder()
	_, n, err = arq.writeSelectiveAck(timestamp)
	if err != nil {
		return Fail, n, err
	}

	if segOrdered != nil {
		copy(buffer, segOrdered.(*Segment).Data)
		return status, len(segOrdered.(*Segment).Data), err
	} else {
		clear(buffer)
		return status, 0, err
	}
}

func (arq *selectiveArq) SetReadTimeout(t time.Duration) {
	arq.extension.SetReadTimeout(t)
}

func clear(b []byte) {
	for k := range b {
		b[k] = 0
	}
}

func (arq *selectiveArq) handleSelectiveAck(seg *Segment, timestamp time.Time) {
	ackedSequenceNumberBitmap := NewBitmap(arq.windowSize).Init(BytesToUint32(seg.Data))

	for i := uint32(0); i < arq.windowSize; i++ {
		ele, _ := ackedSequenceNumberBitmap.Get(i)
		if ele == 1 {
			arq.notAckedSegment[i] = nil
			arq.window--
		}
	}

	for _, ele := range arq.notAckedSegment {
		if ele != nil {
			arq.readyToSendSegmentQueue.PushFront(ele)
		}
	}

	_, _, err := arq.writeQueuedSegments(timestamp)
	if err != nil {
		//TODO
		panic("")
	}
}

func (arq *selectiveArq) writeSelectiveAck(timestamp time.Time) (StatusCode, int, error) {
	ack := CreateSelectiveAckSegment(arq.getAndIncrementCurrentSequenceNumber(), arq.AckedBitmap)
	return arq.extension.Write(ack.Buffer, timestamp)
}
