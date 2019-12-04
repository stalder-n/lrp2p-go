package atp

import (
	"crypto/rand"
	"log"
	"time"
)

var generateRandomSequenceNumber = func() uint32 {
	b := make([]byte, 4)
	_, err := rand.Read(b)
	handleError(err)
	sequenceNum := bytesToUint32(b)
	if sequenceNum == 0 {
		sequenceNum++
	}
	return sequenceNum
}

func hasSegmentTimedOut(seg *segment, time time.Time) bool {
	if seg == nil {
		return false
	}

	timeout := seg.timestamp.Add(retransmissionTimeout)
	return time.After(timeout)
}

type Connector interface {
	Read(buffer []byte, timestamp time.Time) (statusCode, int, error)
	Write(buffer []byte, timestamp time.Time) (statusCode, int, error)
	Open() error
	Close() error
	SetReadTimeout(time.Duration)
}

func connect(connector Connector) Connector {
	//sec := newSecurityExtension(connector, nil, nil)
	arq := newSelectiveArq(generateRandomSequenceNumber(), connector)
	return arq
}

func handleError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
