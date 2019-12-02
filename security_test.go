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
	suite.Suite
	alphaSecurity, betaSecurity   *securityExtension
	alphaConnector, betaConnector *ChannelConnector
}

func (suite *SecurityTestSuite) SetupTest() {
	SegmentMtu = 128
}

func (suite *SecurityTestSuite) TearDownTest() {
	suite.handleTestError(suite.alphaSecurity.Close())
	suite.handleTestError(suite.betaSecurity.Close())
	SegmentMtu = DefaultMTU
}

func (suite *SecurityTestSuite) mockConnections(peerKeyKnown bool) {
	endpoint1, endpoint2 := make(chan []byte, 100), make(chan []byte, 100)
	startTime := time.Now()
	suite.alphaConnector, suite.betaConnector = &ChannelConnector{
		In:            endpoint1,
		Out:           endpoint2,
		artificialNow: startTime,
	}, &ChannelConnector{
		In:            endpoint2,
		Out:           endpoint1,
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

func (suite *SecurityTestSuite) exchangeGreeting() {
	expected := "Hello, World!"
	group := sync.WaitGroup{}
	group.Add(2)
	go func() {
		_, _, _ = suite.alphaSecurity.Write([]byte(expected), suite.alphaConnector.artificialNow)
		buf := make([]byte, SegmentMtu)
		_, n, _ := suite.alphaSecurity.Read(buf, suite.alphaConnector.artificialNow)
		suite.Equal(expected, string(buf[:n]))
		group.Done()
	}()
	go func() {
		buf := make([]byte, SegmentMtu)
		_, n, _ := suite.betaSecurity.Read(buf, suite.alphaConnector.artificialNow)
		suite.Equal(expected, string(buf[:n]))
		_, _, _ = suite.betaSecurity.Write([]byte(expected), suite.alphaConnector.artificialNow)
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
