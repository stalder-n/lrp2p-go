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
	segs := make([]*segment, 0, 5)
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
	segs := make([]*segment, 0, 9)
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

func (suite *SegmentTestSuite) TestCreateEmptyAckSegment() {
	segs := make([]*segment, 0)
	ack := createAckSegment(1, segs)
	suite.Len(ack.data, 1)
	suite.EqualValues([]byte{ackDelimEnd}, ack.data)
}

func (suite *SegmentTestSuite) TestAckToSequenceNums() {
	ack := createFlaggedSegment(1, flagACK, []byte{
		0, 0, 0, 2, ackDelimSeq,
		0, 0, 0, 4, ackDelimSeq,
		0, 0, 0, 6, ackDelimEnd})
	nums := ackSegmentToSequenceNumbers(ack)
	suite.EqualValues([]uint32{1, 2, 4, 6}, nums)
}

func (suite *SegmentTestSuite) TestAckWithRangesToSequenceNums() {
	ack := createFlaggedSegment(1, flagACK, []byte{
		0, 0, 0, 2, ackDelimRange, 0, 0, 0, 3, ackDelimSeq,
		0, 0, 0, 5, ackDelimRange, 0, 0, 0, 7, ackDelimEnd})
	nums := ackSegmentToSequenceNumbers(ack)
	suite.EqualValues([]uint32{1, 2, 3, 5, 6, 7}, nums)
}

func (suite *SegmentTestSuite) TestComplexAckToSequenceNums() {
	ack := createFlaggedSegment(1, flagACK, []byte{
		0, 0, 0, 2, ackDelimSeq,
		0, 0, 0, 4, ackDelimRange, 0, 0, 0, 7, ackDelimSeq,
		0, 0, 0, 9, ackDelimSeq,
		0, 0, 0, 11, ackDelimSeq,
		0, 0, 0, 13, ackDelimRange, 0, 0, 0, 14, ackDelimEnd})
	nums := ackSegmentToSequenceNumbers(ack)
	suite.EqualValues([]uint32{1, 2, 4, 5, 6, 7, 9, 11, 13, 14}, nums)
}

func (suite *SegmentTestSuite) TestEmptyAckToSequenceNums() {
	ack := createFlaggedSegment(1, flagACK, []byte{ackDelimEnd})
	nums := ackSegmentToSequenceNumbers(ack)
	suite.EqualValues([]uint32{1}, nums)
}

func (suite *SegmentTestSuite) TestInsertSegmentInOrder() {
	seg1 := createFlaggedSegment(1, 0, nil)
	seg3 := createFlaggedSegment(3, 0, nil)
	seg4 := createFlaggedSegment(4, 0, nil)
	seg5 := createFlaggedSegment(5, 0, nil)
	seg6 := createFlaggedSegment(6, 0, nil)
	seg7 := createFlaggedSegment(7, 0, nil)
	seg8 := createFlaggedSegment(8, 0, nil)
	seg10 := createFlaggedSegment(10, 0, nil)

	segs := make([]*segment, 0, 10)
	segs = insertSegmentInOrder(segs, seg1)
	segs = insertSegmentInOrder(segs, seg5)
	segs = insertSegmentInOrder(segs, seg10)

	suite.EqualValues(segs, []*segment{seg1, seg5, seg10})

	segs = insertSegmentInOrder(segs, seg7)
	segs = insertSegmentInOrder(segs, seg4)
	segs = insertSegmentInOrder(segs, seg3)

	suite.EqualValues(segs, []*segment{seg1, seg3, seg4, seg5, seg7, seg10})

	segs = insertSegmentInOrder(segs, seg8)
	segs = insertSegmentInOrder(segs, seg6)
	// test insert does not store duplicates
	segs = insertSegmentInOrder(segs, seg6)

	suite.EqualValues(segs, []*segment{seg1, seg3, seg4, seg5, seg6, seg7, seg8, seg10})
}

func (suite *SegmentTestSuite) TestRemoveSegment() {
	seg1 := createFlaggedSegment(1, 0, nil)
	seg3 := createFlaggedSegment(3, 0, nil)
	seg5 := createFlaggedSegment(5, 0, nil)
	seg7 := createFlaggedSegment(7, 0, nil)
	segs := []*segment{seg1, seg3, seg5, seg7}
	removed, segs := removeSegment(segs, 5)
	suite.EqualValues(segs, []*segment{seg1, seg3, seg7})
	suite.Equal(seg5, removed)
	removed, segs = removeSegment(segs, 6)
	suite.EqualValues(segs, []*segment{seg1, seg3, seg7})
	suite.Nil(removed)
}

func TestSegment(t *testing.T) {
	suite.Run(t, &SegmentTestSuite{})
}
