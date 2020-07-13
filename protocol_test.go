package atp

import (
	"github.com/stretchr/testify/suite"
	"net"
	"sync"
	"testing"
	"time"
)

type UDPConnectorTestSuite struct {
	atpTestSuite
	alphaConnection *udpSocket
	betaConnection  *udpSocket
	alphaAddr       *net.UDPAddr
	betaAddr        *net.UDPAddr
}

func (suite *UDPConnectorTestSuite) SetupTest() {
	suite.alphaAddr = createUDPAddress(localhost, 3030)
	suite.betaAddr = createUDPAddress(localhost, 3031)
	alphaConnection, err := udpListen(suite.alphaAddr, testErrorChannel)
	suite.handleTestError(err)
	betaConnection, err := udpListen(suite.betaAddr, testErrorChannel)
	suite.handleTestError(err)
	suite.alphaConnection = alphaConnection
	suite.betaConnection = betaConnection
	suite.timestamp = time.Now()
}

func (suite *UDPConnectorTestSuite) TearDownTest() {
	suite.handleTestError(suite.alphaConnection.Close())
	suite.handleTestError(suite.betaConnection.Close())
}

func (suite *UDPConnectorTestSuite) TestSimpleUDPConnection() {
	expectedAlpha := "Hello beta"
	expectedBeta := "Hello alpha"
	suite.write(suite.alphaConnection, suite.betaAddr, expectedAlpha)
	suite.write(suite.betaConnection, suite.alphaAddr, expectedBeta)
	suite.read(suite.betaConnection, expectedAlpha)
	suite.read(suite.alphaConnection, expectedBeta)
}

func (suite *UDPConnectorTestSuite) write(socket *udpSocket, addr *net.UDPAddr, payload string) {
	status, n, err := socket.Write([]byte(payload), addr)
	suite.Equal(success, status)
	suite.Equal(len(payload), n)
	suite.handleTestError(err)
}

func (suite *UDPConnectorTestSuite) read(socket *udpSocket, expected string) {
	buffer := make([]byte, segmentMtu)
	status, _, n, err := socket.Read(buffer, suite.timestamp)
	suite.Equal(success, status)
	suite.Equal(expected, string(buffer[:n]))
	suite.handleTestError(err)
}

func TestUdpConnector(t *testing.T) {
	suite.Run(t, new(UDPConnectorTestSuite))
}

type PeerSocketTestSuite struct {
	atpTestSuite
	socketA *PeerSocket
	socketB *PeerSocket
}

func (suite *PeerSocketTestSuite) SetupTest() {
	suite.socketA = SocketListen(localhost, 3030)
	suite.socketB = SocketListen(localhost, 3031)
}

func (suite *PeerSocketTestSuite) TearDownTest() {
	suite.handleTestError(suite.socketA.Close())
	suite.handleTestError(suite.socketB.Close())
}

func (suite *PeerSocketTestSuite) TestOneToOneConnection() {
	expectedA := []byte("Hello World A")
	expectedB := []byte("Hello World B")
	mutex := sync.WaitGroup{}
	mutex.Add(2)
	go func() {
		conn := suite.socketA.ConnectTo(localhost, 3031)
		suite.NotNil(conn)
		n, err := conn.Write(expectedA)
		suite.handleTestError(err)
		buffer := make([]byte, 128)
		n, err = conn.Read(buffer)
		suite.Equal(expectedB, buffer[:n])
		suite.handleTestError(err)
		mutex.Done()
	}()
	go func() {
		conn := suite.socketB.Accept()
		suite.NotNil(conn)
		n, err := conn.Write(expectedB)
		suite.handleTestError(err)
		buffer := make([]byte, 128)
		n, err = conn.Read(buffer)
		suite.Equal(expectedA, buffer[:n])
		suite.handleTestError(err)
		mutex.Done()
	}()
	mutex.Wait()
}

func (suite *PeerSocketTestSuite) TestMultiplexing() {
	socketC := SocketListen(localhost, 3032)
	expectedA := []byte("Hello World A")
	expectedB := []byte("Hello World B")
	mutex := sync.WaitGroup{}
	mutex.Add(2)
	go func() {
		conn1 := suite.socketA.ConnectTo(localhost, 3031)
		conn2 := suite.socketA.Accept()
		suite.NotNil(conn1)
		suite.NotNil(conn2)

		n, err := conn2.Write(expectedA)
		suite.handleTestError(err)

		buffer := make([]byte, 128)
		n, err = conn1.Read(buffer)
		suite.Equal(expectedB, buffer[:n])
		suite.handleTestError(err)
		mutex.Done()
	}()
	go func() {
		conn1 := suite.socketB.Accept()
		conn2 := socketC.ConnectTo(localhost, 3030)
		suite.NotNil(conn1)
		suite.NotNil(conn2)

		n, err := conn1.Write(expectedB)
		suite.handleTestError(err)

		buffer := make([]byte, 128)
		n, err = conn2.Read(buffer)
		suite.Equal(expectedA, buffer[:n])
		suite.handleTestError(err)
		mutex.Done()
	}()
	mutex.Wait()
	socketC.Close()
}

func TestSocketConnection(t *testing.T) {
	suite.Run(t, new(PeerSocketTestSuite))
}
