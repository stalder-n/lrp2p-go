package lowlevel

import (
	"github.com/stretchr/testify/suite"
	"testing"
)

type segmentSuite struct {
	suite.Suite
}

func (suite *segmentSuite) TestCreateAckSegment() {
	a := CreateAckSegment(1, 2);
	suite.NotEqual("", a.getDataAsString());
	suite.NotNil(a.Data);
	suite.NotEqual(0, len(a.Data));
	suite.ElementsMatch([]byte{0, 0, 0, 2}, a.Data);
	suite.NotNil(a.SequenceNumber);
	suite.ElementsMatch([]byte{0, 0, 0, 1}, a.SequenceNumber);
	suite.Equal(uint32(2), a.getExpectedSequenceNumber());
}

func (suite *segmentSuite) TestCreateSegment() {
	buffer := []byte{6, 0, 0, 0, 0, 1, 'T', 'E', 'S', 'T'};
	b := CreateSegment(buffer);
	suite.NotNil(b.SequenceNumber);
	suite.ElementsMatch([]byte{0, 0, 0, 1}, b.SequenceNumber);
	suite.Equal("TEST", b.getDataAsString());
	suite.NotNil(b.Data);
	suite.Equal(4, len(b.Data));
	suite.ElementsMatch([]byte{'T', 'E', 'S', 'T'}, b.Data);
}

func (suite *segmentSuite) TestCreateFlaggedSegment() {
	data := []byte{'T', 'E', 'S', 'T'};
	c := createFlaggedSegment(100, 123, data);
	suite.True(c.IsFlaggedAs(123));
	suite.ElementsMatch([]byte{'T', 'E', 'S', 'T'}, c.Data);
	suite.Equal(byte(6), c.getDataOffset());
	suite.Equal(byte(123), c.getFlags());
	suite.Equal("TEST", c.getDataAsString());
	suite.Equal(int(c.getDataOffset()), c.GetHeaderSize());
	suite.ElementsMatch([]byte{0, 0, 0, 100}, c.SequenceNumber);
	suite.Equal(uint32(100), c.GetSequenceNumber());
}

func (suite *segmentSuite) TestGetExpectedSequenceNumber() {
	data := []byte{0, 0, 0, 100, 'T', 'E', 'S', 'T'};
	c := createFlaggedSegment(100, 123, data);
	suite.Equal(uint32(100), c.getExpectedSequenceNumber());
}

func TestSegment(t *testing.T) {
	suite.Run(t, &segmentSuite{})
}
