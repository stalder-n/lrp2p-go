package atp

import (
	"github.com/stretchr/testify/suite"
	"testing"
	"time"
)

type UdpConnectorTestSuite struct {
	atpTestSuite
	alphaConnection *udpConnector
	betaConnection  *udpConnector
}

func (suite *UdpConnectorTestSuite) SetupTest() {
	alphaConnection, err := newUDPConnector("localhost", 3031, 3030, testErrorChannel)
	suite.handleTestError(err)
	betaConnection, err := newUDPConnector("localhost", 3030, 3031, testErrorChannel)
	suite.handleTestError(err)
	suite.alphaConnection = alphaConnection
	suite.betaConnection = betaConnection
}
func (suite *UdpConnectorTestSuite) TearDownTest() {
	err := suite.alphaConnection.Close()
	suite.handleTestError(err)
	err = suite.betaConnection.Close()
	suite.handleTestError(err)
}

func (suite *UdpConnectorTestSuite) TestSimpleGreeting() {
	expectedAlpha := "Hello beta"
	expectedBeta := "Hello alpha"
	timestamp := time.Now()
	suite.write(suite.alphaConnection, expectedAlpha, timestamp)
	suite.write(suite.betaConnection, expectedBeta, timestamp)
	suite.read(suite.betaConnection, expectedAlpha, timestamp)
	suite.read(suite.alphaConnection, expectedBeta, timestamp)
}

func TestUdpConnector(t *testing.T) {
	suite.Run(t, new(UdpConnectorTestSuite))
}
