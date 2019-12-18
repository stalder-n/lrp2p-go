package atp

import (
	"github.com/stretchr/testify/suite"
	"testing"
)

type SegmentTestSuite struct {
	atpTestSuite
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
	c := createFlaggedSegment(100, 123, data)
	suite.True(c.isFlaggedAs(123))
	suite.ElementsMatch([]byte{'T', 'E', 'S', 'T'}, c.data)
	suite.Equal(byte(6), c.getDataOffset())
	suite.Equal(byte(123), c.getFlags())
	suite.Equal("TEST", c.getDataAsString())
	suite.Equal(int(c.getDataOffset()), c.getHeaderSize())
	suite.ElementsMatch([]byte{0, 0, 0, 100}, c.sequenceNumber)
	suite.Equal(uint32(100), c.getSequenceNumber())
}

func (suite *SegmentTestSuite) TestGetExpectedSequenceNumber() {
	data := []byte{0, 0, 0, 100, 'T', 'E', 'S', 'T'}
	c := createFlaggedSegment(100, 123, data)
	suite.Equal(uint32(100), c.getExpectedSequenceNumber())
}

func (suite *SegmentTestSuite) Test_uint32ToBytes() {
	test := uint32ToBytes(0)
	suite.Equal(4, len(test))
}

func TestSegment(t *testing.T) {
	suite.Run(t, &SegmentTestSuite{})
}
