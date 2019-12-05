package atp

import (
	"time"
)

// TODO: sequence diagram
// TODO: slow start, congestion control
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
}

func newSelectiveArq(initialSequenceNumber uint32, extension Connector) *selectiveArq {
	var windowSize uint32 = 20
	arq := &selectiveArq{
		extension:               extension,
		readyToSendSegmentQueue: newQueue(),
		notAckedSegment:         make([]*segment, 32),
		ackedBitmap:             newEmptyBitmap(32),
		currentSequenceNumber:   initialSequenceNumber,
		windowSize:              windowSize,
	}
	return arq
}

func (arq *selectiveArq) getAndIncrementCurrentSequenceNumber() uint32 {
	result := arq.currentSequenceNumber
	arq.currentSequenceNumber++
	return result
}

func (arq *selectiveArq) Open() error {
	return arq.extension.Open()
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

// TODO: Adjust window correctly in this method
func (arq *selectiveArq) writeQueuedSegments(timestamp time.Time) (statusCode, int, error) {
	sumN := 0
	for !arq.readyToSendSegmentQueue.IsEmpty() {
		if arq.window >= arq.windowSize {
			return windowFull, sumN, nil
		}
		seg := arq.readyToSendSegmentQueue.Dequeue().(*segment)
		_, n, err := arq.extension.Write(seg.buffer, timestamp)
		seg.timestamp = time.Now()
		arq.window++

		if err != nil {
			return fail, n, err
		}

		sumN += n
		arq.notAckedSegment[seg.getSequenceNumber()%arq.windowSize] = seg
	}

	return success, sumN, nil
}

func (arq *selectiveArq) queueNewSegments(buffer []byte) {
	if len(buffer) == 0 {
		return
	}
	newSegmentQueue := createSegments(buffer, arq.getAndIncrementCurrentSequenceNumber)
	for !newSegmentQueue.IsEmpty() {
		segment := newSegmentQueue.Dequeue()
		arq.readyToSendSegmentQueue.Enqueue(segment)
	}
}

func (arq *selectiveArq) Write(buffer []byte, timestamp time.Time) (statusCode, int, error) {
	arq.queueNewSegments(buffer)
	arq.queueTimedOutSegmentsForWrite(timestamp)
	status, n, err := arq.writeQueuedSegments(timestamp)
	return status, n, err
}

func (arq *selectiveArq) Read(buffer []byte, timestamp time.Time) (statusCode, int, error) {
	buf := make([]byte, segmentMtu)
	status, n, err := arq.extension.Read(buf, timestamp)
	if err != nil {
		return fail, n, err
	}
	seg := createSegment(buf)

	if seg.isFlaggedAs(flagACK) {
		arq.handleAck(seg, timestamp)
		return ackReceived, n, err
	}

	if seg.getSequenceNumber() < arq.ackedBitmap.sequenceNumber {
		_, _, err = arq.writeAck(timestamp)
		return invalidSegment, 0, nil
	}

	arq.ackedBitmap.Add(seg.getSequenceNumber(), seg)
	segOrdered := arq.ackedBitmap.GetAndRemoveInorder()

	_, n, err = arq.writeAck(timestamp)
	if err != nil {
		return fail, n, err
	}

	if segOrdered != nil {
		copy(buffer, segOrdered.data)
		return status, len(segOrdered.data), err
	} else {
		return invalidSegment, 0, err
	}
}

func (arq *selectiveArq) SetReadTimeout(t time.Duration) {
	arq.extension.SetReadTimeout(t)
}

func (arq *selectiveArq) handleAck(seg *segment, timestamp time.Time) {
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

func (arq *selectiveArq) writeAck(timestamp time.Time) (statusCode, int, error) {
	ack := createAckSegment(arq.getAndIncrementCurrentSequenceNumber(), arq.ackedBitmap)
	return arq.extension.Write(ack.buffer, timestamp)
}
