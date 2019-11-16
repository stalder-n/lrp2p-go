package lowlevel

import (
	"crypto/rand"
	"log"
	"time"
)

type Connector interface {
	Read([]byte) (StatusCode, int, error)
	Write([]byte) (StatusCode, int, error)
	Open() error
	Close() error
}

var SequenceNumberFactory = func() uint32 {
	b := make([]byte, 4)
	_, err := rand.Read(b)
	handleError(err)
	sequenceNum := BytesToUint32(b)
	if sequenceNum == 0 {
		sequenceNum++
	}
	return sequenceNum
}

func HasSegmentTimedOut(seg *Segment) bool {
	if seg == nil {
		return false
	}

	timeout := seg.Timestamp.Add(RetransmissionTimeout)
	return time.Now().After(timeout)
}

func handleError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
