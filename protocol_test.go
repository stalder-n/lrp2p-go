package atp

import (
	"github.com/stretchr/testify/suite"
	"testing"
	"time"
)

type udpConnectorTestSuite struct {
	atpTestSuite
	alphaConnection *udpConnector
	betaConnection  *udpConnector
}

func (suite *udpConnectorTestSuite) SetupTest() {
	alphaConnection, err := newUDPConnector("localhost", 3031, 3030, testErrorChannel)
	suite.handleTestError(err)
	betaConnection, err := newUDPConnector("localhost", 3030, 3031, testErrorChannel)
	suite.handleTestError(err)
	suite.alphaConnection = alphaConnection
	suite.betaConnection = betaConnection
}
func (suite *udpConnectorTestSuite) TearDownTest() {
	err := suite.alphaConnection.Close()
	suite.handleTestError(err)
	err = suite.betaConnection.Close()
	suite.handleTestError(err)
}

func (suite *udpConnectorTestSuite) TestSimpleGreeting() {
	timestamp := time.Now()
	status, n, err := suite.alphaConnection.Write([]byte("Hello beta"), timestamp)
	suite.Equal(success, status)
	suite.Equal(10, n)
	suite.Equal(nil, err)
	status, n, err = suite.betaConnection.Write([]byte("Hello alpha"), timestamp)
	suite.Equal(success, status)
	suite.Equal(11, n)
	suite.Equal(nil, err)
}

func TestUdpConnector(t *testing.T) {
	suite.Run(t, new(udpConnectorTestSuite))
}
