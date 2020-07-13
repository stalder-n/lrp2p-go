package atp

import (
	"github.com/stretchr/testify/suite"
	"sync"
	"testing"
	"time"
)

type SecurityTestSuite struct {
	atpTestSuite
	alphaConn, betaConn *Conn
}

func (suite *SecurityTestSuite) TearDownTest() {
	suite.handleTestError(suite.alphaConn.endpoint.Close())
	suite.handleTestError(suite.betaConn.endpoint.Close())
}

func (suite *SecurityTestSuite) SetupTestWithChannels() {
	endpoint1, endpoint2 := make(chan []byte, 100), make(chan []byte, 100)
	startTime := time.Now()
	suite.timestamp = startTime
	alphaChannel, betaChannel := &channelConnector{
		in:            endpoint1,
		out:           endpoint2,
		artificialNow: startTime,
		timeout:       1 * time.Millisecond,
	}, &channelConnector{
		in:            endpoint2,
		out:           endpoint1,
		artificialNow: startTime,
		timeout:       1 * time.Millisecond,
	}
	suite.alphaConn = newConn(1, alphaChannel)
	suite.alphaConn.enc = newEncryption(testErrorChannel)
	suite.betaConn = newConn(2, betaChannel)
	suite.betaConn.enc = newEncryption(testErrorChannel)
}
func (suite *SecurityTestSuite) SetupTestWithUdp() {
	startTime := time.Now()
	suite.timestamp = startTime
	alphaAddr := createUDPAddress(localhost, 3030)
	betaAddr := createUDPAddress(localhost, 3031)
	alphaConnector, err := udpListen(alphaAddr, testErrorChannel)
	suite.handleTestError(err)
	betaConnector, err := udpListen(betaAddr, testErrorChannel)
	suite.handleTestError(err)
	suite.alphaConn = newConn(1, &udpConnector{socket: alphaConnector, remoteAddr: betaAddr})
	suite.alphaConn.enc = newEncryption(testErrorChannel)
	suite.betaConn = newConn(2, &udpConnector{socket: betaConnector, remoteAddr: alphaAddr})
	suite.betaConn.enc = newEncryption(testErrorChannel)
}

func (suite *SecurityTestSuite) exchangeGreeting() {
	expected := repeatDataSize(1, 1)
	group := sync.WaitGroup{}
	group.Add(3)
	go func() {
		state := suite.alphaConn.enc.InitiateHandshake(suite.alphaConn)
		suite.Equal(connected, state)
		out := make([]byte, segmentMtu)
		n, err := suite.alphaConn.enc.Encrypt([]byte(expected), out)
		suite.handleTestError(err)
		_, _, err = suite.alphaConn.writeToEndpoint(out[:n])
		suite.handleTestError(err)
		group.Done()
	}()
	go func() {
		state := suite.betaConn.enc.AcceptHandshake(suite.betaConn)
		in := make([]byte, segmentMtu)
		out := make([]byte, segmentMtu)
		_, n, err := suite.betaConn.readFromEndpoint(in, suite.timestamp)
		suite.handleTestError(err)
		n, err = suite.betaConn.enc.Decrypt(in[:n], out)
		suite.Equal(expected, string(out[:n]))
		suite.handleTestError(err)
		suite.Equal(connected, state)
		group.Done()
	}()
	go func() {
		suite.readAndNotify(suite.betaConn)
		suite.readAndNotify(suite.alphaConn)
		suite.readAndNotify(suite.betaConn)
		group.Done()
	}()
	group.Wait()
}

func (suite *SecurityTestSuite) readAndNotify(conn *Conn) {
	buffer := make([]byte, 128)
	_, n, err := conn.readFromEndpoint(buffer, suite.timestamp)
	suite.handleTestError(err)
	conn.enc.handshakeMessage = buffer[8:n]
	conn.enc.messageNotifier.Signal()
}

func (suite *SecurityTestSuite) TestExchangeGreetingOverChannels() {
	suite.SetupTestWithChannels()
	suite.exchangeGreeting()
}

func (suite *SecurityTestSuite) TestExchangeGreetingOverUdp() {
	suite.SetupTestWithUdp()
	suite.exchangeGreeting()
}

func TestSecurityExtension(t *testing.T) {
	suite.Run(t, new(SecurityTestSuite))
}
