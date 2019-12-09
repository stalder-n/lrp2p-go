package atp

import (
	"sync"
	"time"
)

// TODO: sequence diagram
// TODO: slow start, congestion control
// TODO: mutex for write operations
type selectiveArq struct {
	extension  Connector
	writeMutex sync.Mutex
	//receiver
	ackedBitmap *bitmap

	//sender
	notAckedSegment         []*segment
	readyToSendSegmentQueue *queue
	currentSequenceNumber   uint32
	window                  uint32
	windowSize              uint32
	errorChannel            chan error
}

func newSelectiveArq(initialSequenceNumber uint32, extension Connector, errors chan error) *selectiveArq {
	var windowSize uint32 = 20
	arq := &selectiveArq{
		extension:               extension,
		readyToSendSegmentQueue: newQueue(),
		notAckedSegment:         make([]*segment, 32),
		ackedBitmap:             newEmptyBitmap(32),
		currentSequenceNumber:   initialSequenceNumber,
		windowSize:              windowSize,
		errorChannel:            errors,
	}
	return arq
}

func (arq *selectiveArq) getAndIncrementCurrentSequenceNumber() uint32 {
	result := arq.currentSequenceNumber
	arq.currentSequenceNumber++
	return result
}

func (arq *selectiveArq) Close() error {
	return arq.extension.Close()
}

func (arq *selectiveArq) addExtension(extension Connector) {
	arq.extension = extension
}

func (arq *selectiveArq) queueTimedOutSegmentsForWrite(timestamp time.Time) {
	for i := 0; i < len(arq.notAckedSegment); i++ {
		seg := arq.notAckedSegment[i]
		if hasSegmentTimedOut(seg, timestamp) {
			arq.readyToSendSegmentQueue.PushFront(seg)
			arq.notAckedSegment[i] = nil
		}
	}
}

func (arq *selectiveArq) queueMissingSegmentsForWrite(ackedSegments *bitmap, timestamp time.Time) {
	startQueueing := false
	for i := arq.windowSize - 1; i >= 0 && i < arq.windowSize; i-- {
		seg := arq.notAckedSegment[ackedSegments.sequenceNumber%arq.windowSize+i]
		if ackedSegments.bitmapData[i] == 1 {
			startQueueing = true
		}
		if ackedSegments.bitmapData[i] == 0 && startQueueing {
			arq.readyToSendSegmentQueue.PushFront(seg)
			arq.notAckedSegment[i] = nil
		}
	}
}

func (arq *selectiveArq) writeQueuedSegments(timestamp time.Time) (statusCode, int, error) {
	sumN := 0
	for !arq.readyToSendSegmentQueue.IsEmpty() {
		if arq.window >= arq.windowSize {
			return windowFull, sumN, nil
		}
		seg := arq.readyToSendSegmentQueue.Dequeue().(*segment)
		_, n, err := arq.extension.Write(seg.buffer, timestamp)
		seg.timestamp = timestamp
		arq.reduceWindow()

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
	arq.writeMutex.Lock()
	defer arq.writeMutex.Unlock()

	arq.queueNewSegments(buffer)
	arq.queueTimedOutSegmentsForWrite(timestamp)
	status, n, err := arq.writeQueuedSegments(timestamp)
	return status, n, err
}

func (arq *selectiveArq) Read(buffer []byte, timestamp time.Time) (statusCode, int, error) {
	buf := make([]byte, segmentMtu)
	segOrdered := arq.ackedBitmap.GetAndRemoveInorder()
	if segOrdered != nil {
		copy(buffer, segOrdered.data)
		return success, len(segOrdered.data), nil
	}

	status, n, err := arq.extension.Read(buf, timestamp)
	if err != nil {
		return fail, n, err
	}
	seg := createSegment(buf)

	if seg.isFlaggedAs(flagACK) {
		arq.handleAck(seg, timestamp)
		return ackReceived, n, err
	}

	arq.ackedBitmap.Add(seg.getSequenceNumber(), seg)
	_, n, err = arq.writeAck(timestamp)
	if err != nil {
		return fail, n, err
	}

	segOrdered = arq.ackedBitmap.GetAndRemoveInorder()
	if segOrdered != nil {
		copy(buffer, segOrdered.data)
		return status, len(segOrdered.data), nil
	} else {
		return invalidSegment, 0, err
	}
}

func (arq *selectiveArq) SetReadTimeout(t time.Duration) {
	arq.extension.SetReadTimeout(t)
}

func (arq *selectiveArq) reportError(err error) {
	if err != nil {
		arq.errorChannel <- err
	}
}

func (arq *selectiveArq) handleAck(seg *segment, timestamp time.Time) {
	arq.writeMutex.Lock()
	defer arq.writeMutex.Unlock()
	ackedBitmap := newBitmap(arq.windowSize, bytesToUint32(seg.data[:4]), bytesToUint32(seg.data[4:]))
	arq.removeAckedSegment(ackedBitmap)
	arq.queueMissingSegmentsForWrite(ackedBitmap, timestamp)
	_, _, err := arq.writeQueuedSegments(timestamp)
	reportError(err)
}

func (arq *selectiveArq) removeAckedSegment(ackedBitmap *bitmap) {
	for i, seg := range arq.notAckedSegment {
		if seg != nil && seg.getSequenceNumber() < ackedBitmap.sequenceNumber {
			arq.notAckedSegment[i] = nil
			arq.increaseWindow()
		}
	}

	for i, bit := range ackedBitmap.bitmapData {
		if bit == 1 {
			index := (ackedBitmap.sequenceNumber + uint32(i)) % arq.windowSize
			arq.notAckedSegment[index] = nil
			arq.increaseWindow()
		}
	}
}

func (arq *selectiveArq) writeAck(timestamp time.Time) (statusCode, int, error) {
	ack := createAckSegment(arq.getAndIncrementCurrentSequenceNumber(), arq.ackedBitmap)
	return arq.extension.Write(ack.buffer, timestamp)
}

func (arq *selectiveArq) reduceWindow() {
	arq.window++
}

func (arq *selectiveArq) increaseWindow() {
	arq.window--
}
