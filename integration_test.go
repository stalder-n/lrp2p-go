package lrp2p

import (
	"github.com/stretchr/testify/suite"
	"sync"
	"testing"
	"time"
)

type IntegrationTestSuite struct {
	lrp2pTestSuite
	alphaSocket                *PeerSocket
	betaSocket                 *PeerSocket
	alphaConnectionManipulator *connectionManipulator
	betaConnectionManipulator  *connectionManipulator
}

func (suite *IntegrationTestSuite) SetupTest() {
	suite.alphaSocket = SocketListen(localhost, 3030)
	suite.betaSocket = SocketListen(localhost, 3031)
}

func (suite *IntegrationTestSuite) TearDownTest() {
	suite.handleTestError(suite.alphaSocket.Close())
	suite.handleTestError(suite.betaSocket.Close())
}

func (suite *IntegrationTestSuite) TestStableLowLatencyConnection() {
	if testing.Short() {
		suite.T().Skip("Skipping integration test")
	}
	writeDelay := 5 * time.Millisecond
	mutex := sync.WaitGroup{}
	mutex.Add(2)
	go func() {
		conn := suite.alphaSocket.ConnectTo(localhost, 3031)
		conn.endpoint = &connectionManipulator{conn: conn.endpoint, writeDelay: writeDelay}
		_, _ = conn.Write([]byte(repeatDataSize(1, 100)))
		_, _ = conn.Write([]byte(repeatDataSize(0, 1)))
		mutex.Done()
		mutex.Wait()
	}()
	go func() {
		readBuffer := make([]byte, getDataChunkSize())
		conn := suite.betaSocket.Accept()
		conn.endpoint = &connectionManipulator{conn: conn.endpoint, writeDelay: writeDelay}
		for {
			n, err := conn.Read(readBuffer)
			suite.handleTestError(err)
			if string(readBuffer[:n]) == repeatDataSizeInc(0, 1) {
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
