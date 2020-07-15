package atp

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestAdd(t *testing.T) {
	fp1 := I(1)
	fp2 := I(2)
	fp3 := fp1.Add(fp2)
	assert.Equal(t, "3,00", fp3.String())
	assert.Equal(t, "2,00", fp2.String())
	assert.Equal(t, "1,00", fp1.String())
}

func TestAddFraction(t *testing.T) {
	fp1 := F(1, 26)
	fp2 := F(2, 0)
	fp3 := fp1.Add(fp2)
	assert.Equal(t, "3,41", fp3.String())
	assert.Equal(t, "2,00", fp2.String())
	assert.Equal(t, "1,41", fp1.String())
}

func TestAddFraction2(t *testing.T) {
	fp1 := D(1, 40)
	fp2 := F(2, 0)
	fp3 := fp1.Add(fp2)
	assert.Equal(t, "3,41", fp3.String())
	assert.Equal(t, "2,00", fp2.String())
	assert.Equal(t, "1,41", fp1.String())
}

func TestAddNeg(t *testing.T) {
	fp1 := I(-3)
	fp2 := I(2)
	fp3 := fp1.Add(fp2)
	assert.Equal(t, "-1,00", fp3.String())
	assert.Equal(t, "2,00", fp2.String())
	assert.Equal(t, "-3,00", fp1.String())
}

func TestAddNegFrac(t *testing.T) {
	fp1 := F(0, 0)
	fp2 := F(0, 2)
	fp3 := fp1.Sub(fp2)
	assert.Equal(t, "-0,04", fp3.String())
	assert.Equal(t, "0,04", fp2.String())
	assert.Equal(t, "0,00", fp1.String())
}

func TestAddNegFrac2(t *testing.T) {
	fp1 := F(0, -1)
	fp2 := F(0, -2)
	fp3 := fp1.Add(fp2)
	assert.Equal(t, "-0,05", fp3.String())
	assert.Equal(t, "-0,04", fp2.String())
	assert.Equal(t, "-0,02", fp1.String())
}

func TestAddNegFracDec(t *testing.T) {
	fp1 := D(0, -50)
	fp2 := D(0, -25)
	fp3 := fp1.Add(fp2)
	assert.Equal(t, "-0,72", fp3.String())
}

func TestMul(t *testing.T) {
	fp1 := I(3)
	fp2 := I(2)
	fp3 := fp1.Mul(fp2)
	assert.Equal(t, "6,00", fp3.String())
	assert.Equal(t, "2,00", fp2.String())
	assert.Equal(t, "3,00", fp1.String())
}

func TestMul2(t *testing.T) {
	fp1 := D(3, 3)
	fp2 := I(2)
	fp3 := fp1.Mul(fp2)
	assert.Equal(t, "6,07", fp3.String())
	assert.Equal(t, "2,00", fp2.String())
	assert.Equal(t, "3,04", fp1.String())
}

func TestMulNeg(t *testing.T) {
	fp1 := D(3, 3)
	fp2 := I(-2)
	fp3 := fp1.Mul(fp2)
	assert.Equal(t, "-6,07", fp3.String())
	assert.Equal(t, "-2,00", fp2.String())
	assert.Equal(t, "3,04", fp1.String())
}

func TestRoot(t *testing.T) {
	fp1 := I(740)
	fp2 := fp1.Root(3)
	assert.Equal(t, "9,04", fp2.String())
	assert.Equal(t, "740,00", fp1.String())
}

func TestRootFrac(t *testing.T) {
	fp1 := D(45, 1)
	fp2 := fp1.Root(3)
	assert.Equal(t, "3,55", fp2.String())
	assert.Equal(t, "45,02", fp1.String())
}

func TestDiv(t *testing.T) {
	fp1 := F(0, -32)
	fp2 := I(5)
	fp3 := fp1.Mul(fp2)
	assert.Equal(t, "-2,50", fp3.String())
	assert.Equal(t, "5,00", fp2.String())
	assert.Equal(t, "-0,50", fp1.String())
}
