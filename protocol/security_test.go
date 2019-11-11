package protocol

import (
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
)

func TestSecurityExtension_SimpleWriteRead(t *testing.T) {
	endpoint1, endpoint2 := make(chan []byte, 100), make(chan []byte, 100)
	connector1, connector2 := &channelConnector{
		in:  endpoint1,
		out: endpoint2,
	}, &channelConnector{
		in:  endpoint2,
		out: endpoint1,
	}
	sec1 := securityExtension{connector: connector1}
	sec2 := securityExtension{connector: connector2}

	expected := "Hello, World!"

	group := sync.WaitGroup{}
	group.Add(2)
	go func() {
		_, _, _ = sec1.Write([]byte(expected))
		group.Done()
	}()

	go func() {
		buf := make([]byte, segmentMtu)
		_, n, _ := sec2.Read(buf)
		assert.Equal(t, expected, string(buf[:n]))
		group.Done()
	}()

	group.Wait()

}
