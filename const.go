package atp

import "time"

const (
	flagACK byte = 1
	flagSYN byte = 2
)
const (
	defaultMTU   = 64
	headerLength = 6
)

type statusCode int

const (
	success statusCode = iota
	fail
	ackReceived
	pendingSegments
	invalidSegment
	windowFull
	waitingForHandshake
	invalidNonce
	timeout
)

type position struct {
	Start int
	End   int
}

var dataOffsetPosition = position{0, 1}
var flagPosition = position{1, 2}
var sequenceNumberPosition = position{2, 6}

var retransmissionTimeout = 200 * time.Millisecond
var timeoutCheckInterval = 100 * time.Millisecond

var timeZero = time.Time{}
