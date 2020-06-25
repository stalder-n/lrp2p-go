package atp

import (
	"github.com/stretchr/testify/suite"
	"sync"
	"testing"
	"time"
)

type IntegrationTestSuite struct {
	atpTestSuite
	alphaSocket                *Socket
	betaSocket                 *Socket
	alphaSegManipulator        *segmentManipulator
	betaSegManipulator         *segmentManipulator
	alphaConnectionManipulator *connectionManipulator
	betaConnectionManipulator  *connectionManipulator
}

func (suite *IntegrationTestSuite) SetupTest() {
	alphaUdp, err := udpListen(localhost, 3030, testErrorChannel)
	suite.handleTestError(err)
	suite.alphaConnectionManipulator = &connectionManipulator{extension: alphaUdp}
	suite.alphaSocket = newSocket(suite.alphaConnectionManipulator, testErrorChannel)

	betaUdp, err := udpListen(localhost, 3031, testErrorChannel)
	suite.handleTestError(err)
	suite.betaConnectionManipulator = &connectionManipulator{extension: betaUdp}
	suite.betaSocket = newSocket(suite.betaConnectionManipulator, testErrorChannel)

	suite.alphaSocket.ConnectTo(localhost, 3031)
	suite.betaSocket.ConnectTo(localhost, 3030)
}

func (suite *IntegrationTestSuite) TearDownTest() {
	suite.handleTestError(suite.alphaSocket.Close())
	suite.handleTestError(suite.betaSocket.Close())
}

func (suite *IntegrationTestSuite) configureConnection(rtt time.Duration) {
	suite.alphaConnectionManipulator.writeDelay = rtt / 2
	suite.betaConnectionManipulator.writeDelay = rtt / 2
}

func (suite *IntegrationTestSuite) TestStableLowLatencyConnection() {
	if testing.Short() {
		suite.T().Skip("Skipping integration test")
	}
	suite.configureConnection(10 * time.Millisecond)
	mutex := sync.WaitGroup{}
	mutex.Add(2)
	go func() {
		_, _ = suite.alphaSocket.Write([]byte(repeatDataSize(1, 100)))
		_, _ = suite.alphaSocket.Write([]byte(repeatDataSize(0, 1)))
		mutex.Done()
		mutex.Wait()
	}()
	go func() {
		readBuffer := make([]byte, getDataChunkSize())
		for {
			_, err := suite.betaSocket.Read(readBuffer)
			suite.handleTestError(err)
			if string(readBuffer) == repeatDataSizeInc(0, 1) {
				break
			}
		}
		mutex.Done()
	}()
	mutex.Wait()
}

func TestIntegration(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}
