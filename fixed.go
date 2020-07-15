package atp

import "fmt"

const (
	one  = Int26_6(1 << 6)
	mask = one - 1
)

// Int26_6 is a signed 26.6 fixed-point number.
//
// The integer part ranges from -33554432 to 33554431, inclusive. The
// fractional part has 6 bits of precision.
//"-33554432:00" // The minimum value is -(1<<25).
//
// For example, the number one-and-a-quarter is Int26_6(1<<6 + 1<<4).
type Int26_6 int32

//from: https://github.com/golang/image/blob/master/math/fixed/fixed.go
func I(i int) Int26_6 {
	return Int26_6(i << 6)
}

//0.4 is for f = 26 -> 0.4 * 64 ~ 26
//https://en.wikibooks.org/wiki/Floating_Point/Fixed-Point_Numbers
//https://github.com/ventolin13/fixed/blob/master/fixed.go
func F(i int, f int) Int26_6 {
	if i == 0 && f < 0 {
		return Int26_6(f)
	} else {
		return Int26_6(i<<6 + (f & 0x3f))
	}
}

//0.5 would be (0, 50), 0.05 would be (0, 5)
func D(i int, d int) Int26_6 {
	if i == 0 && d < 0 {
		return Int26_6(((d * 64) + 99) / 100)
	} else {
		return Int26_6(i<<6 + ((((d * 64) + 99) / 100) & 0x3f))
	}
}

// String returns a human-readable representation of a 26.6 fixed-point number.
//
// For example, the number one-and-a-quarter becomes "1:16".
func (x Int26_6) String() string {
	if x >= 0 {
		return fmt.Sprintf("%d,%02d", x>>6, (((x&mask)*100)+63)/64)
	} else {
		x = -x
		return fmt.Sprintf("-%d,%02d", x>>6, (((x&mask)*100)+63)/64)
	}
}

// Floor returns the greatest integer value less than or equal to x.
//
// Its return type is int, not Int26_6.
func (x Int26_6) Floor() int { return int((x + 0x00) >> 6) }

// Round returns the nearest integer value to x. Ties are rounded up.
//
// Its return type is int, not Int26_6.
func (x Int26_6) Round() int { return int((x + 0x20) >> 6) }

// Ceil returns the least integer value greater than or equal to x.
//
// Its return type is int, not Int26_6.
func (x Int26_6) Ceil() int { return int((x + 0x3f) >> 6) }

// Mul returns x*y in 26.6 fixed-point arithmetic.
func (x Int26_6) Mul(y Int26_6) Int26_6 {
	return Int26_6((int64(x)*int64(y) + 1<<5) >> 6)
}

func (x Int26_6) Add(y Int26_6) Int26_6 {
	return x + y
}

func (x Int26_6) Sub(y Int26_6) Int26_6 {
	return x - y
}

//https://www.thetopsites.net/article/53000795.shtml
func (x Int26_6) Root(n uint32) Int26_6 {
	if n == 0 {
		return 0
	} //error: zeroth root is indeterminate!
	if n == 1 {
		return x
	}

	v := one
	tp := v.Pow(n)
	for tp < x { // first power of two such that v**n >= a
		v <<= 1
		tp = v.Pow(n)
	}

	if tp == x {
		return v
	} // answer is a power of two

	v >>= 1
	bit := v >> 1
	tp = v.Pow(n) // v is highest power of two such that v**n < a
	for x > tp {
		v += bit // add bit to value
		t := v.Pow(n)
		if t > x {
			v -= bit
		} else { // did we add too much?
			tp = t
		}
		bit >>= 1
		if bit == 0 {
			break
		}
	}
	return v // closest integer such that v**n <= a
}

// used by root function...
func (x Int26_6) Pow(e uint32) Int26_6 {
	if e == 0 {
		return one
	}

	r := one
	for e != 0 {
		if (e & 1) == 1 {
			r = r.Mul(x)
		}
		e >>= 1
		x = x.Mul(x)
	}
	return r
}
