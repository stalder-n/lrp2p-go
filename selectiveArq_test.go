package atp

import (
	"github.com/stretchr/testify/suite"
	"testing"
	"time"
)

type ArqTestSuite struct {
	atpTestSuite
	alphaArq, betaArq                 *selectiveArq
	alphaManipulator, betaManipulator *segmentManipulator
}

func newMockSelectiveRepeatArqConnection(connector *channelConnector, name string) (*selectiveArq, *segmentManipulator) {
	manipulator := &segmentManipulator{extension: connector}
	arq := newSelectiveArq(1, manipulator, testErrorChannel)
	return arq, manipulator
}

func (suite *ArqTestSuite) SetupTest() {
	endpoint1, endpoint2 := make(chan []byte, 100), make(chan []byte, 100)
	connector1, connector2 := &channelConnector{
		in:  endpoint1,
		out: endpoint2,
	}, &channelConnector{
		in:  endpoint2,
		out: endpoint1,
	}
	suite.alphaArq, suite.alphaManipulator = newMockSelectiveRepeatArqConnection(connector1, "alpha")
	suite.betaArq, suite.betaManipulator = newMockSelectiveRepeatArqConnection(connector2, "beta")
	segmentMtu = headerLength + 8
}

func (suite *ArqTestSuite) TestSimpleWrite() {
	now := time.Now()
	suite.write(suite.alphaArq, "12345678", now)
	suite.read(suite.betaArq, "12345678", now)
	suite.readAck(suite.alphaArq, now)
}

func (suite *ArqTestSuite) TearDownTest() {
	segmentMtu = defaultMTU
	suite.handleTestError(suite.alphaArq.Close())
	suite.handleTestError(suite.betaArq.Close())
}

func TestSelectiveRepeatArq(t *testing.T) {
	suite.Run(t, new(ArqTestSuite))
}
