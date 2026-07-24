package main

import (
	"math"
	"math/bits"
)

func encodeExponent(biased_exp uint64) uint64 {
	if (biased_exp == uint64(2047)) {
		return uint64(2047)
	}
	if (biased_exp >= uint64(1023)) {
		return (biased_exp - uint64(1023))
	}
	return (uint64(2046) - biased_exp)
}

func decodeExponent(enc uint64) uint64 {
	if (enc == uint64(2047)) {
		return uint64(2047)
	}
	if (enc <= uint64(1023)) {
		return (enc + uint64(1023))
	}
	return (uint64(2046) - enc)
}

func reverseBitsN(val uint64, n uint64) uint64 {
	if (n == uint64(0)) {
		return uint64(0)
	}
	return (bits.Reverse64(val) >> (uint64(64) - n))
}

func updateMantissa(unbiased_exp int64, mantissa uint64) uint64 {
	if (unbiased_exp <= int64(0)) {
		return reverseBitsN(mantissa, uint64(52))
	}
	if (unbiased_exp <= int64(51)) {
		n_frac := uint64((int64(52) - unbiased_exp))
		frac_mask := ((uint64(1) << n_frac) - uint64(1))
		frac := (mantissa & frac_mask)
		return ((mantissa ^ frac) | reverseBitsN(frac, n_frac))
	}
	return mantissa
}

func isSimpleFloat(x float64) bool {
	if ((math.Signbit(x) || math.IsInf(x, 0)) || math.IsNaN(x)) {
		return false
	}
	if (x >= float64(72057594037927936)) {
		return false
	}
	i := uint64(x)
	return ((float64(i) == x) && (i < uint64(72057594037927936)))
}

func floatToIndex(x float64) uint64 {
	if isSimpleFloat(x) {
		return uint64(x)
	}
	bits := math.Float64bits(x)
	biased_exp := ((bits >> uint64(52)) & uint64(2047))
	mantissa := (bits & uint64(4503599627370495))
	unbiased_exp := (int64(biased_exp) - int64(1023))
	mantissa_enc := updateMantissa(unbiased_exp, mantissa)
	exp_enc := encodeExponent(biased_exp)
	return ((uint64(9223372036854775808) | (exp_enc << uint64(52))) | mantissa_enc)
}

func indexToFloat(i uint64) float64 {
	if ((i >> uint64(63)) == uint64(0)) {
		integral := (i & uint64(72057594037927935))
		return float64(integral)
	}
	exp_enc := ((i >> uint64(52)) & uint64(2047))
	biased_exp := decodeExponent(exp_enc)
	mantissa_enc := (i & uint64(4503599627370495))
	unbiased_exp := (int64(biased_exp) - int64(1023))
	mantissa := updateMantissa(unbiased_exp, mantissa_enc)
	return math.Float64frombits(((biased_exp << uint64(52)) | mantissa))
}

func minReversedInRange(lo uint64, hi uint64, n int64) uint64 {
	if (n == int64(0)) {
		return uint64(0)
	}
	k_lo := ((lo + uint64(1)) >> uint64(1))
	k_hi := (hi >> uint64(1))
	if (k_lo <= k_hi) {
		return (minReversedInRange(k_lo, k_hi, (n - int64(1))) * uint64(2))
	}
	return lo
}

func simplestInRange(lo float64, hi float64) float64 {
	c := math.Ceil(lo)
	if ((c <= hi) && (c < float64(72057594037927936))) {
		return c
	}
	lo_bits := math.Float64bits(lo)
	hi_bits := math.Float64bits(hi)
	e_lo := (lo_bits >> uint64(52))
	e_hi := (hi_bits >> uint64(52))
	m_lo := (lo_bits & uint64(4503599627370495))
	m_hi := (hi_bits & uint64(4503599627370495))
	cond := (e_lo >= uint64(1023))
	e := (func() uint64 { if cond { return e_lo }; return e_hi })()
	m_min := (func() uint64 { if cond { return m_lo }; return (func() uint64 { if (e_lo == e_hi) { return m_lo }; return uint64(0) })() })()
	m_max := (func() uint64 { if cond { return (func() uint64 { if (e_hi == e_lo) { return m_hi }; return uint64(4503599627370495) })() }; return m_hi })()
	unbiased := (int64(e) - int64(1023))
	if (unbiased >= int64(52)) {
		return math.Float64frombits(((e << uint64(52)) | m_min))
	}
	n_frac := (func() int64 { if (unbiased <= int64(0)) { return int64(52) }; return (int64(52) - unbiased) })()
	low_mask := ((uint64(1) << n_frac) - uint64(1))
	h := (m_min >> n_frac)
	l_lo := (m_min & low_mask)
	l_hi := (m_max & low_mask)
	m_best := ((h << n_frac) | minReversedInRange(l_lo, l_hi, n_frac))
	return math.Float64frombits(((e << uint64(52)) | m_best))
}
