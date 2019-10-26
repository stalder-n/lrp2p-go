package protocol

import (
	"bytes"
	"testing"
)

func TestCreateAckSegment(t *testing.T) {
	a := createAckSegment(1)
	if a.getDataAsString() == "" {
		t.Error("getDataAsString is empty in AckSegment")
	}

	if a.data == nil || len(a.data) == 0 {
		t.Error("Data is nil")
	}

	if bytes.Compare(a.data, []byte{'2'}) != 0 {
		t.Error("data is not []byte{'2'}", a.data)
	}

	if a.sequenceNumber == nil && bytes.Compare(a.sequenceNumber, []byte{'1'}) != 0 {
		t.Error("sequenceNumber is nil or not equal to []byte{'1'}", a.sequenceNumber)
	}
}

func TestCreateSegment(t *testing.T) {
	buffer := []byte{6, 1, 255, 255, 255, 255, 'T', 'E', 'S', 'T'}
	b := createSegment(buffer)
	if b.getDataAsString() != "TEST" {
		t.Error("getDataAsString is not TEST", b.getDataAsString())
	}

	if b.data == nil || len(b.data) != 4 {
		t.Error("Data is nil or len != 4", len(b.data))
	}

	if bytes.Compare(b.data, []byte{'T', 'E', 'S', 'T'}) != 0 {
		t.Error("data is not []byte{'T', 'E', 'S','T'}", b.data)
	}

	if b.sequenceNumber == nil && bytes.Compare(b.sequenceNumber, []byte{'1'}) != 0 {
		t.Error("sequenceNumber is nil or not equal to []byte{'1'}", b.sequenceNumber)
	}
}

func TestCreateFlaggedSegment(t *testing.T) {
	data := []byte{'T', 'E', 'S', 'T'}
	c := createFlaggedSegment(100, 123, data)

	if !c.isFlaggedAs(123) {
		t.Error("Flag does not contain 123", c.getFlags())
	}

	if bytes.Compare(c.data, []byte{'T', 'E', 'S', 'T'}) != 0 {
		t.Error("data is not []byte{'T','E','S','T'}", c.data)
	}

	if c.getDataOffset() != 6 {
		t.Error("DataOffset is not 6", c.getDataOffset())
	}

	if c.getFlags() != 123 {
		t.Error("Flag is not 123", c.getFlags())
	}

	if c.getDataAsString() == "TEST" {
		t.Error("getDataAsString is not TEST", c.getDataAsString())
	}

	if c.getHeaderSize() != int(c.getDataOffset()) {
		t.Error("getHeaderSize is not getDataOffset", c.getHeaderSize(), c.getDataOffset())
	}

	if c.getSequenceNumber() != 100 {
		t.Error("getSequenceNumber is not 100", c.getSequenceNumber())
	}

	if c.getExpectedSequenceNumber() != 101 {
		t.Error("getExpectedSequenceNumber is not 101", c.getExpectedSequenceNumber())
	}

}
