package goprotocol

import (
	"time"
)

type selectiveArq struct {
	extension Connector
	//receiver
	AckedBitmap *Bitmap

	//sender
	NotAckedSegment         []*Segment
	readyToSendSegmentQueue *Queue
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
	arq.readyToSendSegmentQueue = NewQueue()
	arq.NotAckedSegment = make([]*Segment, arq.windowSize)
	arq.AckedBitmap = NewBitmap(arq.windowSize)

	if arq.sequenceNumberFactory == nil {
		arq.sequenceNumberFactory = SequenceNumberFactory
	}

	arq.currentSequenceNumber = arq.sequenceNumberFactory()

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
	for i := 0; i < len(arq.NotAckedSegment); i++ {
		seg := arq.NotAckedSegment[i]
		if HasSegmentTimedOut(seg) {
			arq.readyToSendSegmentQueue.PushFront(seg)
			arq.NotAckedSegment[i] = nil
		} else {
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
		arq.NotAckedSegment[seg.GetSequenceNumber()%arq.windowSize] = seg
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
	arq.removeAckedSegment(seg.Data)
	arq.queueTimedOutSegmentsForWrite()
	arq.writeQueuedSegments(timestamp)
}

func (arq *selectiveArq) removeAckedSegment(data []byte) {
	ackedSequenceNumberBitmap := NewBitmap(arq.windowSize).Init(BytesToUint32(data))
	for i := uint32(0); i < arq.windowSize; i++ {
		ele, _ := ackedSequenceNumberBitmap.Get(i)
		if ele == 1 {
			index := (ackedSequenceNumberBitmap.SeqNumber + i) % arq.windowSize
			arq.NotAckedSegment[index] = nil
			arq.window--
		}
	}
}

func (arq *selectiveArq) writeSelectiveAck(timestamp time.Time) (StatusCode, int, error) {
	ack := CreateSelectiveAckSegment(arq.getAndIncrementCurrentSequenceNumber(), arq.AckedBitmap)
	return arq.extension.Write(ack.Buffer, timestamp)
}
