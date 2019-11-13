package lowlevel

import "time"

const (
	FlagACK          byte = 1
	FlagSYN          byte = 2
	FlagSelectiveACK byte = 4
)
const (
	DefaultMTU   = 64
	HeaderLength = 6
)

type StatusCode int

const (
	Success StatusCode = iota
	Fail
	AckReceived
	pendingSegments
	InvalidSegment
	WindowFull
)

type Position struct {
	Start int
	End   int
}

var DataoffsetPosition = Position{0, 1}
var FlagPosition = Position{1, 2}
var SequencenumberPosition = Position{2, 6}

var RetransmissionTimeout = 200 * time.Millisecond

func Max(first uint32, second uint32) uint32 {
	var max uint32

	if first >= second {
		max = first
	} else {
		max = second
	}

	return max
}
