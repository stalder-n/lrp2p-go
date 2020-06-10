package atp

import (
	"crypto/rand"
	"github.com/flynn/noise"
	"github.com/stretchr/testify/suite"
	"sync"
	"testing"
	"time"
)

type SecurityTestSuite struct {
	atpTestSuite
	alphaSecurity, betaSecurity   *securityExtension
	alphaConnector, betaConnector *channelConnector
}

func (suite *SecurityTestSuite) TearDownTest() {
	suite.handleTestError(suite.alphaSecurity.Close())
	suite.handleTestError(suite.betaSecurity.Close())
}

func (suite *SecurityTestSuite) mockConnections(peerKeyKnown bool) {
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
		suite.alphaSecurity = newSecurityExtension(suite.alphaConnector, &alphaKey, betaKey.Public, testErrorChannel)
		suite.betaSecurity = newSecurityExtension(suite.betaConnector, &betaKey, alphaKey.Public, testErrorChannel)
	} else {
		suite.alphaSecurity = newSecurityExtension(suite.alphaConnector, &alphaKey, nil, testErrorChannel)
		suite.betaSecurity = newSecurityExtension(suite.betaConnector, &betaKey, nil, testErrorChannel)
	}
}

func (suite *SecurityTestSuite) exchangeGreeting() {
	//	expected := "Hello, World!"
	expected := repeatDataSize(1, 1)
	group := sync.WaitGroup{}
	group.Add(2)
	go func() {
		suite.write(suite.alphaSecurity, expected, suite.alphaConnector.artificialNow)
		suite.read(suite.alphaSecurity, expected, suite.alphaConnector.artificialNow)
		group.Done()
	}()
	go func() {
		suite.read(suite.betaSecurity, expected, suite.betaConnector.artificialNow)
		suite.write(suite.betaSecurity, expected, suite.betaConnector.artificialNow)
		group.Done()
	}()
	group.Wait()
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
