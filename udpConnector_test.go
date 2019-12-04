package atp

import (
	"github.com/stretchr/testify/suite"
	"testing"
	"time"
)

type UdpConnectorTestSuite struct {
	suite.Suite
	alphaConnection *udpConnector
	betaConnection  *udpConnector
}

func (suite *UdpConnectorTestSuite) SetupTest() {
	alphaConnection, err := newUdpConnector("localhost", 3031, 3030)
	handleTestError(suite.T(), err)
	betaConnection, err := newUdpConnector("localhost", 3030, 3031)
	handleTestError(suite.T(), err)
	suite.alphaConnection = alphaConnection
	suite.betaConnection = betaConnection
}
func (suite *UdpConnectorTestSuite) TearDownTest() {
	err := suite.alphaConnection.Close()
	handleTestError(suite.T(), err)
	err = suite.betaConnection.Close()
	handleTestError(suite.T(), err)
}

func (suite *UdpConnectorTestSuite) TestUdpConnector_SimpleGreeting() {
	timestamp := time.Now()
	status, n, err := suite.alphaConnection.Write([]byte("Hello beta"), timestamp)
	suite.Equal(Success, status)
	suite.Equal(10, n)
	suite.Equal(nil, err)
	status, n, err = suite.betaConnection.Write([]byte("Hello alpha"), timestamp)
	suite.Equal(Success, status)
	suite.Equal(11, n)
	suite.Equal(nil, err)
}

func TestUdpConnector(t *testing.T) {
	suite.Run(t, new(UdpConnectorTestSuite))
}
