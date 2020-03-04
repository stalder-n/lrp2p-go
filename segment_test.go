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

func (suite *SegmentTestSuite) TestCreateAckSegment() {
	segs := make([]*segment, 0, 3)
	segs = append(segs, createFlaggedSegment(2, 0, []byte{}))
	segs = append(segs, createFlaggedSegment(4, 0, []byte{}))
	segs = append(segs, createFlaggedSegment(6, 0, []byte{}))

	ack := createAckSegment(1, segs)
	suite.Len(ack.data, 15)
	suite.EqualValues([]byte{
		0, 0, 0, 2, ackDelimSeq,
		0, 0, 0, 4, ackDelimSeq,
		0, 0, 0, 6, ackDelimEnd}, ack.data)
}

func (suite *SegmentTestSuite) TestCreateAckSegmentWithRanges() {
	segs := make([]*segment, 0, 3)
	segs = append(segs, createFlaggedSegment(2, 0, []byte{}))
	segs = append(segs, createFlaggedSegment(3, 0, []byte{}))
	segs = append(segs, createFlaggedSegment(5, 0, []byte{}))
	segs = append(segs, createFlaggedSegment(6, 0, []byte{}))
	segs = append(segs, createFlaggedSegment(7, 0, []byte{}))

	ack := createAckSegment(1, segs)
	suite.Len(ack.data, 20)
	suite.EqualValues([]byte{
		0, 0, 0, 2, ackDelimRange, 0, 0, 0, 3, ackDelimSeq,
		0, 0, 0, 5, ackDelimRange, 0, 0, 0, 7, ackDelimEnd}, ack.data)
}

func (suite *SegmentTestSuite) TestCreateComplexAckSegment() {
	segs := make([]*segment, 0, 3)
	segs = append(segs, createFlaggedSegment(2, 0, []byte{}))
	segs = append(segs, createFlaggedSegment(4, 0, []byte{}))
	segs = append(segs, createFlaggedSegment(5, 0, []byte{}))
	segs = append(segs, createFlaggedSegment(6, 0, []byte{}))
	segs = append(segs, createFlaggedSegment(7, 0, []byte{}))
	segs = append(segs, createFlaggedSegment(9, 0, []byte{}))
	segs = append(segs, createFlaggedSegment(11, 0, []byte{}))
	segs = append(segs, createFlaggedSegment(13, 0, []byte{}))
	segs = append(segs, createFlaggedSegment(14, 0, []byte{}))

	ack := createAckSegment(1, segs)
	suite.Len(ack.data, 35)
	suite.EqualValues([]byte{
		0, 0, 0, 2, ackDelimSeq,
		0, 0, 0, 4, ackDelimRange, 0, 0, 0, 7, ackDelimSeq,
		0, 0, 0, 9, ackDelimSeq,
		0, 0, 0, 11, ackDelimSeq,
		0, 0, 0, 13, ackDelimRange, 0, 0, 0, 14, ackDelimEnd}, ack.data)
}

func TestSegment(t *testing.T) {
	suite.Run(t, &SegmentTestSuite{})
}
