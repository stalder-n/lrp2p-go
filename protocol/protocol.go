package protocol

import (
	"crypto/rand"
	"log"
	"time"
)

type Connector interface {
	Read([]byte) (statusCode, int, error)
	Write([]byte) (statusCode, int, error)
	Open() error
	Close() error
}

var sequenceNumberFactory = func() uint32 {
	b := make([]byte, 4)
	_, err := rand.Read(b)
	handleError(err)
	sequenceNum := bytesToUint32(b)
	if sequenceNum == 0 {
		sequenceNum++
	}
	return sequenceNum
}

func hasSegmentTimedOut(seg *segment) bool {
	timeout := seg.timestamp.Add(RetransmissionTimeout)
	return time.Now().After(timeout)
}

func Connect(connector Connector) Connector {
	arq := &goBackNArq{}
	arq.AddExtension(connector)
	return arq
}

func UdpConnect(address string, senderPort, receiverPort int) Connector {
	var connector Connector = &udpConnector{
		senderAddress: address,
		senderPort:    senderPort,
		receiverPort:  receiverPort,
	}
	return Connect(connector)
}

func handleError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
