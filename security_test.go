package atp

import (
	"crypto/rand"
	"github.com/flynn/noise"
	"github.com/stretchr/testify/suite"
	"sync"
	"testing"
	"time"
)

type securityTestSuite struct {
	atpTestSuite
	alphaSecurity, betaSecurity   *securityExtension
	alphaConnector, betaConnector *channelConnector
}

func (suite *securityTestSuite) SetupTest() {
	segmentMtu = 128
}

func (suite *securityTestSuite) TearDownTest() {
	suite.handleTestError(suite.alphaSecurity.Close())
	suite.handleTestError(suite.betaSecurity.Close())
	segmentMtu = defaultMTU
}

func (suite *securityTestSuite) mockConnections(peerKeyKnown bool) {
	endpoint1, endpoint2 := make(chan []byte, 100), make(chan []byte, 100)
	startTime := time.Now()
	suite.alphaConnector, suite.betaConnector = &channelConnector{
		in:            endpoint1,
		out:           endpoint2,
		artificialNow: startTime,
	}, &channelConnector{
		in:            endpoint2,
		out:           endpoint1,
		artificialNow: startTime,
	}

	cipherSuite := noise.NewCipherSuite(noise.DH25519, noise.CipherAESGCM, noise.HashBLAKE2b)
	alphaKey, _ := cipherSuite.GenerateKeypair(rand.Reader)
	betaKey, _ := cipherSuite.GenerateKeypair(rand.Reader)
	if peerKeyKnown {
		suite.alphaSecurity = newSecurityExtension(suite.alphaConnector, &alphaKey, betaKey.Public)
		suite.betaSecurity = newSecurityExtension(suite.betaConnector, &betaKey, alphaKey.Public)
	} else {
		suite.alphaSecurity = newSecurityExtension(suite.alphaConnector, &alphaKey, nil)
		suite.betaSecurity = newSecurityExtension(suite.betaConnector, &betaKey, nil)
	}
}

func (suite *securityTestSuite) exchangeGreeting() {
	expected := "Hello, World!"
	group := sync.WaitGroup{}
	group.Add(2)
	go func() {
		_, _, _ = suite.alphaSecurity.Write([]byte(expected), suite.alphaConnector.artificialNow)
		buf := make([]byte, segmentMtu)
		_, n, _ := suite.alphaSecurity.Read(buf, suite.alphaConnector.artificialNow)
		suite.Equal(expected, string(buf[:n]))
		group.Done()
	}()
	go func() {
		buf := make([]byte, segmentMtu)
		_, n, _ := suite.betaSecurity.Read(buf, suite.alphaConnector.artificialNow)
		suite.Equal(expected, string(buf[:n]))
		_, _, _ = suite.betaSecurity.Write([]byte(expected), suite.alphaConnector.artificialNow)
		group.Done()
	}()
	group.Wait()
}

func (suite *securityTestSuite) TestExchangeGreeting() {
	suite.mockConnections(false)
	suite.exchangeGreeting()
}

func (suite *securityTestSuite) TestExchangeGreetingWithKnownPeerKey() {
	suite.mockConnections(true)
	suite.exchangeGreeting()
}

func TestSecurityExtension(t *testing.T) {
	suite.Run(t, new(securityTestSuite))
}
