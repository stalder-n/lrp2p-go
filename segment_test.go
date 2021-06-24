package lrp2p

import (
	"github.com/stretchr/testify/suite"
	"testing"
)

type SegmentTestSuite struct {
	lrp2pTestSuite
}

func (suite *SegmentTestSuite) TestCreateSegment() {
	buffer := []byte{6, 0, 0, 0, 0, 1, 'T', 'E', 'S', 'T'}
	b := createSegment(buffer)
	suite.NotNil(b.sequenceNumber)
	suite.ElementsMatch([]byte{0, 0, 0, 1}, b.sequenceNumber)
	suite.Equal("TEST", b.getDataAsString())
	suite.NotNil(b.data)
	suite.Equal(4, len(b.data))
	suite.ElementsMatch([]byte{'T', 'E', 'S', 'T'}, b.data)
}

func (suite *SegmentTestSuite) TestCreateFlaggedSegment() {
	data := []byte{'T', 'E', 'S', 'T'}
	c := createFlaggedSegment(100, flagSYN, data)
	suite.True(c.isFlaggedAs(flagSYN))
	suite.ElementsMatch([]byte{'T', 'E', 'S', 'T'}, c.data)
	suite.Equal(byte(6), c.getDataOffset())
	suite.Equal(byte(flagSYN), c.getFlags())
	suite.Equal("TEST", c.getDataAsString())
	suite.Equal(int(c.getDataOffset()), c.getHeaderSize())
	suite.ElementsMatch([]byte{0, 0, 0, 100}, c.sequenceNumber)
	suite.Equal(uint32(100), c.getSequenceNumber())
}

func TestSegment(t *testing.T) {
	suite.Run(t, &SegmentTestSuite{})
}
