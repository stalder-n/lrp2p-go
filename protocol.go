package goprotocol

import (
	"crypto/rand"
	"log"
	"time"
)

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

func HasSegmentTimedOut(seg *Segment, time time.Time) bool {
	if seg == nil {
		return false
	}

	timeout := seg.Timestamp.Add(RetransmissionTimeout)
	return time.After(timeout)
}

type Connector interface {
	Read(buffer []byte, timestamp time.Time) (StatusCode, int, error)
	Write(buffer []byte, timestamp time.Time) (StatusCode, int, error)
	Open() error
	Close() error
	SetReadTimeout(time.Duration)
}

func connect(connector Connector) Connector {
	sec := newSecurityExtension(connector, nil, nil)
	arq := newGoBackNArq(sec)
	return arq
}

func handleError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
