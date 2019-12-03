package atp

import (
	"time"
)

type selectiveArq struct {
	extension Connector
	//receiver
	ackedBitmap *bitmap

	//sender
	notAckedSegment         []*segment
	readyToSendSegmentQueue *queue
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
	arq.readyToSendSegmentQueue = newQueue()
	arq.notAckedSegment = make([]*segment, arq.windowSize)
	arq.ackedBitmap = newEmptyBitmap(arq.windowSize)

	if arq.sequenceNumberFactory == nil {
		arq.sequenceNumberFactory = sequenceNumberFactory
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

func (arq *selectiveArq) queueTimedOutSegmentsForWrite(time time.Time) {
	for i := 0; i < len(arq.notAckedSegment); i++ {
		seg := arq.notAckedSegment[i]
		if hasSegmentTimedOut(seg, time) {
			arq.readyToSendSegmentQueue.PushFront(seg)
			arq.notAckedSegment[i] = nil
		} else {
		}
	}
}

func (arq *selectiveArq) writeQueuedSegments(timestamp time.Time) (statusCode, int, error) {
	sumN := 0
	for !arq.readyToSendSegmentQueue.IsEmpty() {
		seg := arq.readyToSendSegmentQueue.Dequeue().(*segment)
		_, n, err := arq.extension.Write(seg.buffer, timestamp)
		seg.timestamp = time.Now()

		if err != nil {
			return fail, n, err
		}

		sumN += n
		arq.notAckedSegment[seg.getSequenceNumber()%arq.windowSize] = seg
	}

	return success, sumN, nil
}

func (arq *selectiveArq) Write(buffer []byte, timestamp time.Time) (statusCode, int, error) {
	newSegmentQueue := createSegments(buffer, arq.getAndIncrementCurrentSequenceNumber)

	oldWindow := arq.window
	for !newSegmentQueue.IsEmpty() && arq.window < arq.windowSize {
		ele := newSegmentQueue.Dequeue()
		arq.readyToSendSegmentQueue.Enqueue(ele)
		arq.window++
	}

	arq.queueTimedOutSegmentsForWrite(timestamp)

	status, _, err := arq.writeQueuedSegments(timestamp)

	if arq.window == arq.windowSize {
		return windowFull, int(arq.window - oldWindow), nil
	}

	return status, int(arq.window - oldWindow), err
}

func (arq *selectiveArq) Read(buffer []byte, timestamp time.Time) (statusCode, int, error) {
	buf := make([]byte, segmentMtu)
	status, n, err := arq.extension.Read(buf, timestamp)
	if err != nil {
		return fail, n, err
	}
	seg := createSegment(buf)

	if seg.isFlaggedAs(flagACK) {
		arq.handleSelectiveAck(seg, timestamp)
		copy(buffer, seg.data)
		return ackReceived, n, err
	}

	//received data-segment:
	arq.ackedBitmap.Add(seg.getSequenceNumber(), seg)

	segOrdered := arq.ackedBitmap.GetAndRemoveInorder()
	_, n, err = arq.writeSelectiveAck(timestamp)
	if err != nil {
		return fail, n, err
	}

	if segOrdered != nil {
		copy(buffer, segOrdered.data)
		return status, len(segOrdered.data), err
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

func (arq *selectiveArq) handleSelectiveAck(seg *segment, timestamp time.Time) {
	arq.removeAckedSegment(seg.data)
	arq.queueTimedOutSegmentsForWrite(timestamp)
	_, _, err := arq.writeQueuedSegments(timestamp)
	reportError(err)
}

func (arq *selectiveArq) removeAckedSegment(data []byte) {
	ackedSequenceNumberBitmap := newBitmap(arq.windowSize, bytesToUint32(data[0:4]), bytesToUint32(data[4:]))

	//due to slide of receiver we have to adjust our array
	for index, ele := range arq.notAckedSegment {
		if ele != nil && ele.getSequenceNumber() < ackedSequenceNumberBitmap.sequenceNumber {
			arq.notAckedSegment[index] = nil
		}
	}

	//remove out of order acked segment
	for i, ele := range ackedSequenceNumberBitmap.bitmapData {
		if ele == 1 {
			index := (ackedSequenceNumberBitmap.sequenceNumber + uint32(i)) % arq.windowSize
			arq.notAckedSegment[index] = nil
			arq.window--
		}
	}
}

func (arq *selectiveArq) writeSelectiveAck(timestamp time.Time) (statusCode, int, error) {
	ack := createSelectiveAckSegment(arq.getAndIncrementCurrentSequenceNumber(), arq.ackedBitmap)
	return arq.extension.Write(ack.buffer, timestamp)
}
