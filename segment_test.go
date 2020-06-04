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

func (suite *SegmentTestSuite) TestRemoveSegmentsAllWhere() {
	seg1 := createFlaggedSegment(1, 0, nil)
	seg3 := createFlaggedSegment(3, 0, nil)
	seg5 := createFlaggedSegment(5, 0, nil)
	seg7 := createFlaggedSegment(7, 0, nil)
	segs := []*segment{seg1, seg3, seg5, seg7}
	removed, segs := removeAllSegmentsWhere(segs, func(seg *segment) bool {
		return seg.getSequenceNumber() < 5
	})
	suite.EqualValues(segs, []*segment{seg5, seg7})
	suite.EqualValues(removed, []*segment{seg1, seg3})
}

func TestSegment(t *testing.T) {
	suite.Run(t, &SegmentTestSuite{})
}
