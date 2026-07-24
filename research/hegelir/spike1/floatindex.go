package main

import (
	"math"
	"math/bits"
)

// Native Go port of hegel-c/src/native/core/float_index.rs.
//
// This is what an IR backend would EMIT for the Go target: idiomatic Go,
// no FFI, no libhegel. It must reproduce the Rust reference bit-for-bit.

func encodeExponent(biasedExp uint64) uint64 {
	if biasedExp == 2047 {
		return 2047
	}
	if biasedExp >= 1023 {
		return biasedExp - 1023
	}
	return 2046 - biasedExp
}

func decodeExponent(enc uint64) uint64 {
	if enc == 2047 {
		return 2047
	}
	if enc <= 1023 {
		return enc + 1023
	}
	return 2046 - enc
}

func reverseBitsN(v, n uint64) uint64 {
	if n == 0 {
		return 0
	}
	return bits.Reverse64(v) >> (64 - n)
}

func updateMantissa(unbiasedExp int64, mantissa uint64) uint64 {
	if unbiasedExp <= 0 {
		return reverseBitsN(mantissa, 52)
	} else if unbiasedExp <= 51 {
		nFrac := uint64(52 - unbiasedExp)
		fracMask := (uint64(1) << nFrac) - 1
		frac := mantissa & fracMask
		return (mantissa ^ frac) | reverseBitsN(frac, nFrac)
	}
	return mantissa
}

func isSimpleFloat(v float64) bool {
	if math.Signbit(v) || math.IsInf(v, 0) || math.IsNaN(v) {
		return false
	}
	// Rust computes `v as u64` (saturating) then checks `i < 2^56`. Any v >= 2^56
	// fails that check, so guard here — this also sidesteps Go's UNDEFINED
	// float64->uint64 conversion for out-of-range inputs (the divergence trap).
	if v >= float64(uint64(1)<<56) {
		return false
	}
	i := uint64(v) // safe: v is in [0, 2^56)
	return float64(i) == v && i < (uint64(1)<<56)
}

func floatToIndex(v float64) uint64 {
	if isSimpleFloat(v) {
		return uint64(v)
	}
	b := math.Float64bits(v)
	biasedExp := (b >> 52) & 0x7FF
	mantissa := b & ((uint64(1) << 52) - 1)
	unbiasedExp := int64(biasedExp) - 1023
	mantissaEnc := updateMantissa(unbiasedExp, mantissa)
	expEnc := encodeExponent(biasedExp)
	return (uint64(1) << 63) | (expEnc << 52) | mantissaEnc
}

func indexToFloat(i uint64) float64 {
	if i>>63 == 0 {
		integral := i & ((uint64(1) << 56) - 1)
		return float64(integral)
	}
	expEnc := (i >> 52) & 0x7FF
	biasedExp := decodeExponent(expEnc)
	mantissaEnc := i & ((uint64(1) << 52) - 1)
	unbiasedExp := int64(biasedExp) - 1023
	mantissa := updateMantissa(unbiasedExp, mantissaEnc)
	return math.Float64frombits((biasedExp << 52) | mantissa)
}

func simplestInRange(lo, hi float64) float64 {
	const mantissaMask = (uint64(1) << 52) - 1
	c := math.Ceil(lo)
	if c <= hi && c < float64(uint64(1)<<56) {
		return c
	}
	loBits := math.Float64bits(lo)
	hiBits := math.Float64bits(hi)
	eLo := loBits >> 52
	eHi := hiBits >> 52
	mLo := loBits & mantissaMask
	mHi := hiBits & mantissaMask

	var e, mMin, mMax uint64
	if eLo >= 1023 {
		e, mMin = eLo, mLo
		if eHi == eLo {
			mMax = mHi
		} else {
			mMax = mantissaMask
		}
	} else {
		e = eHi
		if eLo == eHi {
			mMin = mLo
		} else {
			mMin = 0
		}
		mMax = mHi
	}

	unbiased := int64(e) - 1023
	var mBest uint64
	if unbiased >= 52 {
		mBest = mMin
	} else {
		var nFrac uint32
		if unbiased <= 0 {
			nFrac = 52
		} else {
			nFrac = uint32(52 - unbiased)
		}
		lowMask := (uint64(1) << nFrac) - 1
		h := mMin >> nFrac
		lLo := mMin & lowMask
		lHi := mMax & lowMask
		mBest = (h << nFrac) | minReversedInRange(lLo, lHi, nFrac)
	}
	return math.Float64frombits((e << 52) | mBest)
}

func minReversedInRange(lo, hi uint64, n uint32) uint64 {
	if n == 0 {
		return 0
	}
	kLo := (lo + 1) / 2 // div_ceil(2)
	kHi := hi / 2
	if kLo <= kHi {
		return minReversedInRange(kLo, kHi, n-1) * 2
	}
	return lo
}
