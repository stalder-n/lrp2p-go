package goprotocol

import (
	"crypto/rand"
	"github.com/flynn/noise"
	"github.com/stretchr/testify/suite"
	"sync"
	"testing"
)

type SecurityTestSuite struct {
	suite.Suite
	alphaConnection, betaConnection *securityExtension
}

func (suite *SecurityTestSuite) SetupTest() {
	SegmentMtu = 128
}

func (suite *SecurityTestSuite) TearDownTest() {
	suite.handleTestError(suite.alphaConnection.Close())
	suite.handleTestError(suite.betaConnection.Close())
	SegmentMtu = DefaultMTU
}

func (suite *SecurityTestSuite) mockConnections(peerKeyKnown bool) {
	endpoint1, endpoint2 := make(chan []byte, 100), make(chan []byte, 100)
	alphaConnector, betaConnector := &ChannelConnector{
		In:  endpoint1,
		Out: endpoint2,
	}, &ChannelConnector{
		In:  endpoint2,
		Out: endpoint1,
	}

	cipherSuite := noise.NewCipherSuite(noise.DH25519, noise.CipherAESGCM, noise.HashBLAKE2b)
	alphaKey, _ := cipherSuite.GenerateKeypair(rand.Reader)
	betaKey, _ := cipherSuite.GenerateKeypair(rand.Reader)
	if peerKeyKnown {
		suite.alphaConnection = newSecurityExtension(alphaConnector, &alphaKey, betaKey.Public)
		suite.betaConnection = newSecurityExtension(betaConnector, &betaKey, alphaKey.Public)
	} else {
		suite.alphaConnection = newSecurityExtension(alphaConnector, &alphaKey, nil)
		suite.betaConnection = newSecurityExtension(betaConnector, &betaKey, nil)
	}
}

func (suite *SecurityTestSuite) exchangeGreeting() {
	expected := "Hello, World!"
	group := sync.WaitGroup{}
	group.Add(2)
	go func() {
		_, _, _ = suite.alphaConnection.Write([]byte(expected))
		buf := make([]byte, SegmentMtu)
		_, n, _ := suite.alphaConnection.Read(buf)
		suite.Equal(expected, string(buf[:n]))
		group.Done()
	}()
	go func() {
		buf := make([]byte, SegmentMtu)
		_, n, _ := suite.betaConnection.Read(buf)
		suite.Equal(expected, string(buf[:n]))
		_, _, _ = suite.betaConnection.Write([]byte(expected))
		group.Done()
	}()
	group.Wait()
}

func (suite *SecurityTestSuite) handleTestError(err error) {
	if err != nil {
		suite.Errorf(err, "Error occurred")
	}
}
func (suite *SecurityTestSuite) TestExchangeGreeting() {
	suite.mockConnections(false)
	suite.exchangeGreeting()
}

func (suite *SecurityTestSuite) TestExchangeGreetingWithKnownPeerKey() {
	suite.mockConnections(true)
	suite.exchangeGreeting()
}

func TestSecurityExtension(t *testing.T) {
	suite.Run(t, new(SecurityTestSuite))
}
