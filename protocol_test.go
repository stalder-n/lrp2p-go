package atp

import (
	"github.com/stretchr/testify/suite"
	"sync"
	"testing"
	"time"
)

type UDPConnectorTestSuite struct {
	atpTestSuite
	alphaConnection *udpConnector
	betaConnection  *udpConnector
}

func (suite *UDPConnectorTestSuite) SetupTest() {
	alphaConnection, err := udpListen(3030, testErrorChannel)
	suite.handleTestError(err)
	betaConnection, err := udpListen(3031, testErrorChannel)
	suite.handleTestError(err)
	alphaConnection.ConnectTo("localhost", 3031)
	betaConnection.ConnectTo("localhost", 3030)
	suite.alphaConnection = alphaConnection
	suite.betaConnection = betaConnection
}

func (suite *UDPConnectorTestSuite) TearDownTest() {
	suite.handleTestError(suite.alphaConnection.Close())
	suite.handleTestError(suite.betaConnection.Close())
}

func (suite *UDPConnectorTestSuite) TestSimpleGreeting() {
	expectedAlpha := "Hello beta"
	expectedBeta := "Hello alpha"
	timestamp := time.Now()
	suite.write(suite.alphaConnection, expectedAlpha, timestamp)
	suite.write(suite.betaConnection, expectedBeta, timestamp)
	suite.read(suite.betaConnection, expectedAlpha, timestamp)
	suite.read(suite.alphaConnection, expectedBeta, timestamp)
}

func TestUdpConnector(t *testing.T) {
	suite.Run(t, new(UDPConnectorTestSuite))
}

type SecureConnectionTestSuite struct {
	atpTestSuite
	alphaConnection *securityExtension
	betaConnection  *securityExtension
}

func (suite *SecureConnectionTestSuite) SetupTest() {
	alphaConnector, err := udpListen(3030, testErrorChannel)
	suite.handleTestError(err)
	betaConnector, err := udpListen(3031, testErrorChannel)
	suite.handleTestError(err)
	alphaConnector.ConnectTo("localhost", 3031)
	betaConnector.ConnectTo("localhost", 3030)
	suite.alphaConnection = newSecurityExtension(alphaConnector, nil, nil, testErrorChannel)
	suite.betaConnection = newSecurityExtension(betaConnector, nil, nil, testErrorChannel)
}

func (suite *SecureConnectionTestSuite) TearDownTest() {
	suite.handleTestError(suite.alphaConnection.Close())
	suite.handleTestError(suite.betaConnection.Close())
}

func (suite *SecureConnectionTestSuite) TestSimpleGreeting() {
	expectedAlpha := "Hello beta"
	expectedBeta := "Hello alpha"
	timestamp := time.Now()
	mutex := sync.WaitGroup{}
	mutex.Add(2)
	go func() {
		suite.write(suite.alphaConnection, expectedAlpha, timestamp)
		suite.read(suite.alphaConnection, expectedBeta, timestamp)
		mutex.Done()
	}()
	go func() {
		suite.read(suite.betaConnection, expectedAlpha, timestamp)
		suite.write(suite.betaConnection, expectedBeta, timestamp)
		mutex.Done()
	}()
	mutex.Wait()
}

func TestSecureConnection(t *testing.T) {
	suite.Run(t, new(SecureConnectionTestSuite))
}

type ArqConnectionTestSuite struct {
	atpTestSuite
	alphaConnection *selectiveArq
	betaConnection  *selectiveArq
}

func (suite *ArqConnectionTestSuite) SetupTest() {
	alphaConnector, err := udpListen(3030, testErrorChannel)
	suite.handleTestError(err)
	betaConnector, err := udpListen(3031, testErrorChannel)
	suite.handleTestError(err)
	alphaConnector.ConnectTo("localhost", 3031)
	betaConnector.ConnectTo("localhost", 3030)
	suite.alphaConnection = newSelectiveArq(1, alphaConnector, testErrorChannel)
	suite.betaConnection = newSelectiveArq(1, betaConnector, testErrorChannel)
}

func (suite *ArqConnectionTestSuite) TearDownTest() {
	suite.handleTestError(suite.alphaConnection.Close())
	suite.handleTestError(suite.betaConnection.Close())
}

func (suite *ArqConnectionTestSuite) TestSimpleGreeting() {
	suite.alphaConnection.ackThreshold = 1
	suite.betaConnection.ackThreshold = 1
	expectedAlpha := "Hello beta"
	expectedBeta := "Hello alpha"
	timestamp := time.Now()
	suite.write(suite.alphaConnection, expectedAlpha, timestamp)
	suite.read(suite.betaConnection, expectedAlpha, timestamp)
	suite.readAck(suite.alphaConnection, timestamp)

	suite.write(suite.betaConnection, expectedBeta, timestamp)
	suite.read(suite.alphaConnection, expectedBeta, timestamp)
	suite.readAck(suite.betaConnection, timestamp)
}

func TestArqConnection(t *testing.T) {
	suite.Run(t, new(ArqConnectionTestSuite))
}

type FullConnectionTestSuite struct {
	atpTestSuite
	alphaConnection connector
	betaConnection  connector
}

func (suite *FullConnectionTestSuite) SetupTest() {
	alphaConnector, err := udpListen(3030, testErrorChannel)
	suite.handleTestError(err)
	betaConnector, err := udpListen(3031, testErrorChannel)
	suite.handleTestError(err)
	alphaConnector.ConnectTo("localhost", 3031)
	betaConnector.ConnectTo("localhost", 3030)
	suite.alphaConnection = connect(alphaConnector, testErrorChannel)
	suite.betaConnection = connect(betaConnector, testErrorChannel)

}

func (suite *FullConnectionTestSuite) TearDownTest() {
	suite.handleTestError(suite.alphaConnection.Close())
	suite.handleTestError(suite.betaConnection.Close())
}

func (suite *FullConnectionTestSuite) TestSimpleGreeting() {
	expectedAlpha := "Hello beta"
	expectedBeta := "Hello alpha"
	timestamp := time.Now()
	mutex := sync.WaitGroup{}
	mutex.Add(2)
	go func() {
		suite.write(suite.alphaConnection, expectedAlpha, timestamp)
		suite.readAck(suite.alphaConnection, timestamp)
		suite.read(suite.alphaConnection, expectedBeta, timestamp)
		suite.readExpectStatus(suite.alphaConnection, timeout, suite.timeout())
		mutex.Done()
	}()
	go func() {
		suite.read(suite.betaConnection, expectedAlpha, timestamp)
		suite.readExpectStatus(suite.betaConnection, timeout, suite.timeout())
		suite.write(suite.betaConnection, expectedBeta, timestamp)
		suite.readAck(suite.betaConnection, timestamp)
		mutex.Done()
	}()
	mutex.Wait()
}

func TestFullConnection(t *testing.T) {
	suite.Run(t, new(FullConnectionTestSuite))
}

type SocketTestSuite struct {
	atpTestSuite
	alphaSocket *Socket
	betaSocket  *Socket
}

func (suite *SocketTestSuite) SetupTest() {
	suite.alphaSocket = SocketListen(3030)
	suite.betaSocket = SocketListen(3031)
	suite.alphaSocket.ConnectTo("localhost", 3031)
	suite.betaSocket.ConnectTo("localhost", 3030)
}

func (suite *SocketTestSuite) TearDownTest() {
	suite.handleTestError(suite.alphaSocket.Close())
	suite.handleTestError(suite.betaSocket.Close())
}

func (suite *SocketTestSuite) TestSimpleGreeting() {
	expectedAlpha := "Hello beta"
	expectedBeta := "Hello alpha"
	mutex := sync.WaitGroup{}
	mutex.Add(2)
	go func() {
		n, err := suite.alphaSocket.Write([]byte(expectedAlpha))
		suite.handleTestError(err)
		suite.Equal(len(expectedAlpha), n)

		readBuffer := make([]byte, segmentMtu)
		n, err = suite.alphaSocket.Read(readBuffer)
		suite.handleTestError(err)
		suite.Equal(len(expectedBeta), n)
		mutex.Done()
	}()
	go func() {
		readBuffer := make([]byte, segmentMtu)
		n, err := suite.betaSocket.Read(readBuffer)
		suite.handleTestError(err)
		suite.Equal(len(expectedAlpha), n)

		n, err = suite.betaSocket.Write([]byte(expectedBeta))
		suite.handleTestError(err)
		suite.Equal(len(expectedBeta), n)
		mutex.Done()
	}()
	mutex.Wait()
}

func TestSocketConnection(t *testing.T) {
	suite.Run(t, new(SocketTestSuite))
}
