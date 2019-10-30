package protocol

import "time"

const (
	FlagACK byte = 1
	FlagSYN byte = 2
	FlagEND byte = 4
)
const (
	DefaultMTU   = 64
	HeaderLength = 6
)

type statusCode int

const (
	success statusCode = iota
	fail
	ackReceived
	pendingSegments
	invalidSegment
	windowFull
)

type Position struct {
	Start int
	End   int
}

var DataoffsetPosition = Position{0, 1}
var FlagPosition = Position{1, 2}
var SequencenumberPosition = Position{2, 6}

var RetransmissionTimeout = 200 * time.Millisecond
