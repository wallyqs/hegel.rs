// Reference vector generator for the float_index spike.
//
// The float_index functions below are copied VERBATIM from
// hegel-c/src/native/core/float_index.rs (only the internal debug-assert
// macros are swapped for std debug_assert!). This program calls the real
// algorithm over a curated + randomized input set and prints golden vectors
// that the Go port must reproduce bit-for-bit.

// ---- verbatim from hegel-c/src/native/core/float_index.rs ----

pub fn encode_exponent(biased_exp: u64) -> u64 {
    if biased_exp == 2047 {
        return 2047;
    }
    if biased_exp >= 1023 {
        biased_exp - 1023
    } else {
        2046 - biased_exp
    }
}

pub fn decode_exponent(enc: u64) -> u64 {
    if enc == 2047 {
        return 2047;
    }
    if enc <= 1023 { enc + 1023 } else { 2046 - enc }
}

pub fn reverse_bits_n(v: u64, n: u64) -> u64 {
    if n == 0 {
        return 0;
    }
    v.reverse_bits() >> (64 - n)
}

pub fn update_mantissa(unbiased_exp: i64, mantissa: u64) -> u64 {
    if unbiased_exp <= 0 {
        reverse_bits_n(mantissa, 52)
    } else if unbiased_exp <= 51 {
        let n_frac = (52 - unbiased_exp) as u64;
        let frac_mask = (1u64 << n_frac) - 1;
        let frac = mantissa & frac_mask;
        (mantissa ^ frac) | reverse_bits_n(frac, n_frac)
    } else {
        mantissa
    }
}

fn is_simple_float(v: f64) -> bool {
    if v.is_sign_negative() || !v.is_finite() {
        return false;
    }
    let i = v as u64;
    i as f64 == v && i < (1u64 << 56)
}

pub fn float_to_index(v: f64) -> u64 {
    debug_assert!(!v.is_sign_negative(), "float_to_index called on negative");
    debug_assert!(!v.is_nan(), "float_to_index called on NaN");
    if is_simple_float(v) {
        return v as u64;
    }
    let bits = v.to_bits();
    let biased_exp = (bits >> 52) & 0x7FF;
    let mantissa = bits & ((1u64 << 52) - 1);
    let unbiased_exp = biased_exp as i64 - 1023;
    let mantissa_enc = update_mantissa(unbiased_exp, mantissa);
    let exp_enc = encode_exponent(biased_exp);
    (1u64 << 63) | (exp_enc << 52) | mantissa_enc
}

pub fn index_to_float(i: u64) -> f64 {
    if i >> 63 == 0 {
        let integral = i & ((1u64 << 56) - 1);
        return integral as f64;
    }
    let exp_enc = (i >> 52) & 0x7FF;
    let biased_exp = decode_exponent(exp_enc);
    let mantissa_enc = i & ((1u64 << 52) - 1);
    let unbiased_exp = biased_exp as i64 - 1023;
    let mantissa = update_mantissa(unbiased_exp, mantissa_enc);
    f64::from_bits((biased_exp << 52) | mantissa)
}

pub fn simplest_in_range(lo: f64, hi: f64) -> f64 {
    debug_assert!(lo > 0.0 && lo <= hi && hi.is_finite());
    const MANTISSA_MASK: u64 = (1u64 << 52) - 1;
    let c = lo.ceil();
    if c <= hi && c < (1u64 << 56) as f64 {
        return c;
    }
    let lo_bits = lo.to_bits();
    let hi_bits = hi.to_bits();
    let e_lo = lo_bits >> 52;
    let e_hi = hi_bits >> 52;
    let m_lo = lo_bits & MANTISSA_MASK;
    let m_hi = hi_bits & MANTISSA_MASK;
    let (e, m_min, m_max) = if e_lo >= 1023 {
        (e_lo, m_lo, if e_hi == e_lo { m_hi } else { MANTISSA_MASK })
    } else {
        debug_assert!(e_hi < 1023);
        (e_hi, if e_lo == e_hi { m_lo } else { 0 }, m_hi)
    };
    let unbiased = e as i64 - 1023;
    let m_best = if unbiased >= 52 {
        m_min
    } else {
        let n_frac = if unbiased <= 0 { 52 } else { (52 - unbiased) as u32 };
        let low_mask = (1u64 << n_frac) - 1;
        let h = m_min >> n_frac;
        debug_assert_eq!(m_max >> n_frac, h);
        let l_lo = m_min & low_mask;
        let l_hi = m_max & low_mask;
        (h << n_frac) | min_reversed_in_range(l_lo, l_hi, n_frac)
    };
    f64::from_bits((e << 52) | m_best)
}

fn min_reversed_in_range(lo: u64, hi: u64, n: u32) -> u64 {
    if n == 0 {
        debug_assert_eq!((lo, hi), (0, 0));
        return 0;
    }
    let k_lo = lo.div_ceil(2);
    let k_hi = hi / 2;
    if k_lo <= k_hi {
        min_reversed_in_range(k_lo, k_hi, n - 1) * 2
    } else {
        debug_assert_eq!(lo, hi);
        lo
    }
}

// ---- vector generation ----

// splitmix64: a tiny deterministic generator so the input set is fixed
// without pulling in a dependency. Only used to pick inputs; the outputs
// come from the verbatim functions above.
struct Split(u64);
impl Split {
    fn next(&mut self) -> u64 {
        self.0 = self.0.wrapping_add(0x9E3779B97F4A7C15);
        let mut z = self.0;
        z = (z ^ (z >> 30)).wrapping_mul(0xBF58476D1CE4E5B9);
        z = (z ^ (z >> 27)).wrapping_mul(0x94D049BB133111EB);
        z ^ (z >> 31)
    }
}

fn emit_f2i(v: f64) {
    // Precondition of float_to_index: non-negative, non-NaN.
    let v = v.abs();
    if v.is_nan() {
        return;
    }
    println!("F2I\t{:016x}\t{:016x}", v.to_bits(), float_to_index(v));
}

fn emit_i2f(i: u64) {
    println!("I2F\t{:016x}\t{:016x}", i, index_to_float(i).to_bits());
}

fn emit_sir(a: f64, b: f64) {
    // Precondition: 0 < lo <= hi, both finite.
    if !a.is_finite() || !b.is_finite() || a <= 0.0 || b <= 0.0 {
        return;
    }
    let (lo, hi) = if a <= b { (a, b) } else { (b, a) };
    println!(
        "SIR\t{:016x}\t{:016x}\t{:016x}",
        lo.to_bits(),
        hi.to_bits(),
        simplest_in_range(lo, hi).to_bits()
    );
}

fn main() {
    // Curated edge cases.
    let fixed: &[f64] = &[
        0.0, 1.0, 2.0, 3.0, 0.5, 1.5, 0.1, 1.1, 2.5, 100.0,
        ((1u64 << 56) - 1) as f64,
        (1u64 << 56) as f64,
        1e300, f64::MAX, 5e-324, 2.2250738585072014e-308, 0.3, 123456789.0,
        f64::INFINITY,
    ];
    for &v in fixed {
        emit_f2i(v);
        // round-trip: index_to_float(float_to_index(v))
        emit_i2f(float_to_index(v.abs()));
    }
    for i in [0u64, 1, 2, 3, (1 << 56) - 1, 1 << 63, (1 << 63) | 1, (1 << 63) | (2047 << 52)] {
        emit_i2f(i);
    }
    for (a, b) in [
        (1.0, 2.0), (0.3, 0.7), (1.1, 1.9), (0.5, 0.5), (1e-10, 1e10), (2.0, 2.0),
        (0.9999999, 1.0000001), (1.5, 1.5),
    ] {
        emit_sir(a, b);
    }

    // Randomized inputs.
    let mut rng = Split(0x1234_5678_9ABC_DEF0);
    for _ in 0..4000 {
        let v = f64::from_bits(rng.next());
        emit_f2i(v);
        emit_i2f(rng.next());
    }
    for _ in 0..2000 {
        let a = f64::from_bits(rng.next() & 0x7FFF_FFFF_FFFF_FFFF);
        let b = f64::from_bits(rng.next() & 0x7FFF_FFFF_FFFF_FFFF);
        emit_sir(a, b);
    }
}
