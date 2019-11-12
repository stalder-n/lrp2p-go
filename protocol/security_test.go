package protocol

import (
	"github.com/stretchr/testify/suite"
	"sync"
	"testing"
)

type SecurityTestSuite struct {
	suite.Suite
	alphaConnection, betaConnection *securityExtension
}

func (suite *SecurityTestSuite) SetupTest() {
	suite.mockConnections()
}

func (suite *SecurityTestSuite) TearDownTest() {
	suite.handleTestError(suite.alphaConnection.Close())
	suite.handleTestError(suite.betaConnection.Close())
	setSegmentMtu(defaultSegmentMtu)
}

func (suite *SecurityTestSuite) mockConnections() {
	endpoint1, endpoint2 := make(chan []byte, 100), make(chan []byte, 100)
	connector1, connector2 := &channelConnector{
		in:  endpoint1,
		out: endpoint2,
	}, &channelConnector{
		in:  endpoint2,
		out: endpoint1,
	}
	suite.alphaConnection = &securityExtension{connector: connector1}
	suite.betaConnection = &securityExtension{connector: connector2}
}

func (suite *SecurityTestSuite) handleTestError(err error) {
	if err != nil {
		suite.Errorf(err, "Error occurred")
	}
}

func (suite *SecurityTestSuite) TestSecurityExtension_ExchangeGreeting() {
	suite.mockConnections()
	expected := "Hello, World!"

	group := sync.WaitGroup{}
	group.Add(2)
	go func() {
		_, _, _ = suite.alphaConnection.Write([]byte(expected))
		buf := make([]byte, segmentMtu)
		_, n, _ := suite.alphaConnection.Read(buf)
		suite.Equal(expected, string(buf[:n]))
		group.Done()
	}()

	go func() {
		buf := make([]byte, segmentMtu)
		_, n, _ := suite.betaConnection.Read(buf)
		suite.Equal(expected, string(buf[:n]))
		_, _, _ = suite.betaConnection.Write([]byte(expected))
		group.Done()
	}()

	group.Wait()

}

func TestSecurityExtension(t *testing.T) {
	suite.Run(t, new(SecurityTestSuite))
}
