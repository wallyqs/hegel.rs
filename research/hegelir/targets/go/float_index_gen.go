package main

import (
	"math"
	"math/bits"
)

func encodeExponent(biased_exp uint64) uint64 {
	var c2047 uint64
	var is2047 bool
	var c1023 uint64
	var ge bool
	var r uint64
	var c2046 uint64
	var r2 uint64
	c2047 = uint64(2047)
	is2047 = (biased_exp == c2047)
	if is2047 {
		return c2047
	}
	c1023 = uint64(1023)
	ge = (biased_exp >= c1023)
	if ge {
		r = (biased_exp - c1023)
		return r
	}
	c2046 = uint64(2046)
	r2 = (c2046 - biased_exp)
	return r2
}

func decodeExponent(enc uint64) uint64 {
	var c2047 uint64
	var is2047 bool
	var c1023 uint64
	var le bool
	var r uint64
	var c2046 uint64
	var r2 uint64
	c2047 = uint64(2047)
	is2047 = (enc == c2047)
	if is2047 {
		return c2047
	}
	c1023 = uint64(1023)
	le = (enc <= c1023)
	if le {
		r = (enc + c1023)
		return r
	}
	c2046 = uint64(2046)
	r2 = (c2046 - enc)
	return r2
}

func reverseBitsN(val uint64, n uint64) uint64 {
	var c0 uint64
	var isz bool
	var c64 uint64
	var rev uint64
	var sh uint64
	var r uint64
	c0 = uint64(0)
	isz = (n == c0)
	if isz {
		return c0
	}
	c64 = uint64(64)
	rev = bits.Reverse64(val)
	sh = (c64 - n)
	r = (rev >> sh)
	return r
}

func updateMantissa(unbiased_exp int64, mantissa uint64) uint64 {
	var zero_i int64
	var le0 bool
	var c52 uint64
	var r uint64
	var c51i int64
	var le51 bool
	var c52i int64
	var nfi int64
	var n_frac uint64
	var one uint64
	var shifted uint64
	var frac_mask uint64
	var frac uint64
	var xored uint64
	var revfrac uint64
	zero_i = int64(0)
	le0 = (unbiased_exp <= zero_i)
	if le0 {
		c52 = uint64(52)
		r = reverseBitsN(mantissa, c52)
		return r
	}
	c51i = int64(51)
	le51 = (unbiased_exp <= c51i)
	if le51 {
		c52i = int64(52)
		nfi = (c52i - unbiased_exp)
		n_frac = uint64(nfi)
		one = uint64(1)
		shifted = (one << n_frac)
		frac_mask = (shifted - one)
		frac = (mantissa & frac_mask)
		xored = (mantissa ^ frac)
		revfrac = reverseBitsN(frac, n_frac)
		r = (xored | revfrac)
		return r
	}
	return mantissa
}

func isSimpleFloat(x float64) bool {
	var sb bool
	var inf bool
	var nan bool
	var sb_or_inf bool
	var bad bool
	var false bool
	var pow56f float64
	var ge bool
	var i uint64
	var back float64
	var eqv bool
	var pow56 uint64
	var lt bool
	var res bool
	sb = math.Signbit(x)
	inf = math.IsInf(x, 0)
	nan = math.IsNaN(x)
	sb_or_inf = (sb || inf)
	bad = (sb_or_inf || nan)
	false = false
	if bad {
		return false
	}
	pow56f = float64(72057594037927936)
	ge = (x >= pow56f)
	if ge {
		return false
	}
	i = uint64(x)
	back = float64(i)
	eqv = (back == x)
	pow56 = uint64(72057594037927936)
	lt = (i < pow56)
	res = (eqv && lt)
	return res
}

func floatToIndex(x float64) uint64 {
	var simple bool
	var si uint64
	var bits uint64
	var c52 uint64
	var shr uint64
	var c2047 uint64
	var biased_exp uint64
	var mmask uint64
	var mantissa uint64
	var be_i int64
	var c1023i int64
	var unbiased_exp int64
	var mantissa_enc uint64
	var exp_enc uint64
	var bit63 uint64
	var expsh uint64
	var hi uint64
	var r uint64
	simple = isSimpleFloat(x)
	if simple {
		si = uint64(x)
		return si
	}
	bits = math.Float64bits(x)
	c52 = uint64(52)
	shr = (bits >> c52)
	c2047 = uint64(2047)
	biased_exp = (shr & c2047)
	mmask = uint64(4503599627370495)
	mantissa = (bits & mmask)
	be_i = int64(biased_exp)
	c1023i = int64(1023)
	unbiased_exp = (be_i - c1023i)
	mantissa_enc = updateMantissa(unbiased_exp, mantissa)
	exp_enc = encodeExponent(biased_exp)
	bit63 = uint64(9223372036854775808)
	expsh = (exp_enc << c52)
	hi = (bit63 | expsh)
	r = (hi | mantissa_enc)
	return r
}

func indexToFloat(i uint64) float64 {
	var c63 uint64
	var top uint64
	var c0 uint64
	var isint bool
	var m56 uint64
	var integral uint64
	var f float64
	var c52 uint64
	var shr uint64
	var c2047 uint64
	var exp_enc uint64
	var biased_exp uint64
	var mmask uint64
	var mantissa_enc uint64
	var be_i int64
	var c1023i int64
	var unbiased_exp int64
	var mantissa uint64
	var besh uint64
	var bitsr uint64
	var f2 float64
	c63 = uint64(63)
	top = (i >> c63)
	c0 = uint64(0)
	isint = (top == c0)
	if isint {
		m56 = uint64(72057594037927935)
		integral = (i & m56)
		f = float64(integral)
		return f
	}
	c52 = uint64(52)
	shr = (i >> c52)
	c2047 = uint64(2047)
	exp_enc = (shr & c2047)
	biased_exp = decodeExponent(exp_enc)
	mmask = uint64(4503599627370495)
	mantissa_enc = (i & mmask)
	be_i = int64(biased_exp)
	c1023i = int64(1023)
	unbiased_exp = (be_i - c1023i)
	mantissa = updateMantissa(unbiased_exp, mantissa_enc)
	besh = (biased_exp << c52)
	bitsr = (besh | mantissa)
	f2 = math.Float64frombits(bitsr)
	return f2
}

func minReversedInRange(lo uint64, hi uint64, n int64) uint64 {
	var zero_i int64
	var isz bool
	var c0 uint64
	var one uint64
	var lop1 uint64
	var k_lo uint64
	var k_hi uint64
	var le bool
	var onei int64
	var nm1 int64
	var rec uint64
	var two uint64
	var r uint64
	zero_i = int64(0)
	isz = (n == zero_i)
	c0 = uint64(0)
	if isz {
		return c0
	}
	one = uint64(1)
	lop1 = (lo + one)
	k_lo = (lop1 >> one)
	k_hi = (hi >> one)
	le = (k_lo <= k_hi)
	if le {
		onei = int64(1)
		nm1 = (n - onei)
		rec = minReversedInRange(k_lo, k_hi, nm1)
		two = uint64(2)
		r = (rec * two)
		return r
	}
	return lo
}

func simplestInRange(lo float64, hi float64) float64 {
	var c float64
	var cle bool
	var pow56f float64
	var clt bool
	var inrange bool
	var lo_bits uint64
	var hi_bits uint64
	var c52 uint64
	var e_lo uint64
	var e_hi uint64
	var mmask uint64
	var m_lo uint64
	var m_hi uint64
	var c1023 uint64
	var cond bool
	var ncond bool
	var e uint64
	var m_min uint64
	var m_max uint64
	var eq_hilo bool
	var ne_hilo bool
	var eq_lohi bool
	var ne_lohi bool
	var zero uint64
	var c1023i int64
	var e_i int64
	var unbiased int64
	var c52i int64
	var ge52 bool
	var esh0 uint64
	var bitsr0 uint64
	var f0 float64
	var zero_i int64
	var le0 bool
	var n_frac int64
	var nle0 bool
	var sub int64
	var one uint64
	var shl1 uint64
	var low_mask uint64
	var h uint64
	var l_lo uint64
	var l_hi uint64
	var minrev uint64
	var hsh uint64
	var m_best uint64
	var esh uint64
	var bitsr uint64
	var f float64
	c = math.Ceil(lo)
	cle = (c <= hi)
	pow56f = float64(72057594037927936)
	clt = (c < pow56f)
	inrange = (cle && clt)
	if inrange {
		return c
	}
	lo_bits = math.Float64bits(lo)
	hi_bits = math.Float64bits(hi)
	c52 = uint64(52)
	e_lo = (lo_bits >> c52)
	e_hi = (hi_bits >> c52)
	mmask = uint64(4503599627370495)
	m_lo = (lo_bits & mmask)
	m_hi = (hi_bits & mmask)
	c1023 = uint64(1023)
	cond = (e_lo >= c1023)
	ncond = (!cond)
	e = e_lo
	m_min = m_lo
	m_max = m_hi
	eq_hilo = (e_hi == e_lo)
	ne_hilo = (!eq_hilo)
	if ne_hilo {
		m_max = mmask
	}
	if ncond {
		e = e_hi
		m_max = m_hi
		eq_lohi = (e_lo == e_hi)
		ne_lohi = (!eq_lohi)
		if ne_lohi {
			zero = uint64(0)
			m_min = zero
		}
	}
	c1023i = int64(1023)
	e_i = int64(e)
	unbiased = (e_i - c1023i)
	c52i = int64(52)
	ge52 = (unbiased >= c52i)
	if ge52 {
		esh0 = (e << c52)
		bitsr0 = (esh0 | m_min)
		f0 = math.Float64frombits(bitsr0)
		return f0
	}
	zero_i = int64(0)
	le0 = (unbiased <= zero_i)
	n_frac = c52i
	nle0 = (!le0)
	if nle0 {
		sub = (c52i - unbiased)
		n_frac = sub
	}
	one = uint64(1)
	shl1 = (one << n_frac)
	low_mask = (shl1 - one)
	h = (m_min >> n_frac)
	l_lo = (m_min & low_mask)
	l_hi = (m_max & low_mask)
	minrev = minReversedInRange(l_lo, l_hi, n_frac)
	hsh = (h << n_frac)
	m_best = (hsh | minrev)
	esh = (e << c52)
	bitsr = (esh | m_best)
	f = math.Float64frombits(bitsr)
	return f
}
