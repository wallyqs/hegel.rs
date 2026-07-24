
fn encode_exponent(biased_exp: u64) -> u64 {
    let mut c2047: u64;
    let mut is2047: bool;
    let mut c1023: u64;
    let mut ge: bool;
    let mut r: u64;
    let mut c2046: u64;
    let mut r2: u64;
    c2047 = 2047u64;
    is2047 = (biased_exp == c2047);
    if is2047 {
        return c2047;
    }
    c1023 = 1023u64;
    ge = (biased_exp >= c1023);
    if ge {
        r = (biased_exp - c1023);
        return r;
    }
    c2046 = 2046u64;
    r2 = (c2046 - biased_exp);
    return r2;
}

fn decode_exponent(enc: u64) -> u64 {
    let mut c2047: u64;
    let mut is2047: bool;
    let mut c1023: u64;
    let mut le: bool;
    let mut r: u64;
    let mut c2046: u64;
    let mut r2: u64;
    c2047 = 2047u64;
    is2047 = (enc == c2047);
    if is2047 {
        return c2047;
    }
    c1023 = 1023u64;
    le = (enc <= c1023);
    if le {
        r = (enc + c1023);
        return r;
    }
    c2046 = 2046u64;
    r2 = (c2046 - enc);
    return r2;
}

fn reverse_bits_n(val: u64, n: u64) -> u64 {
    let mut c0: u64;
    let mut isz: bool;
    let mut c64: u64;
    let mut rev: u64;
    let mut sh: u64;
    let mut r: u64;
    c0 = 0u64;
    isz = (n == c0);
    if isz {
        return c0;
    }
    c64 = 64u64;
    rev = val.reverse_bits();
    sh = (c64 - n);
    r = (rev >> sh);
    return r;
}

fn update_mantissa(unbiased_exp: i64, mantissa: u64) -> u64 {
    let mut zero_i: i64;
    let mut le0: bool;
    let mut c52: u64;
    let mut r: u64;
    let mut c51i: i64;
    let mut le51: bool;
    let mut c52i: i64;
    let mut nfi: i64;
    let mut n_frac: u64;
    let mut one: u64;
    let mut shifted: u64;
    let mut frac_mask: u64;
    let mut frac: u64;
    let mut xored: u64;
    let mut revfrac: u64;
    zero_i = 0i64;
    le0 = (unbiased_exp <= zero_i);
    if le0 {
        c52 = 52u64;
        r = reverse_bits_n(mantissa, c52);
        return r;
    }
    c51i = 51i64;
    le51 = (unbiased_exp <= c51i);
    if le51 {
        c52i = 52i64;
        nfi = (c52i - unbiased_exp);
        n_frac = (nfi as u64);
        one = 1u64;
        shifted = (one << n_frac);
        frac_mask = (shifted - one);
        frac = (mantissa & frac_mask);
        xored = (mantissa ^ frac);
        revfrac = reverse_bits_n(frac, n_frac);
        r = (xored | revfrac);
        return r;
    }
    return mantissa;
}

fn is_simple_float(x: f64) -> bool {
    let mut sb: bool;
    let mut inf: bool;
    let mut nan: bool;
    let mut sb_or_inf: bool;
    let mut bad: bool;
    let mut ff: bool;
    let mut pow56f: f64;
    let mut ge: bool;
    let mut i: u64;
    let mut back: f64;
    let mut eqv: bool;
    let mut pow56: u64;
    let mut lt: bool;
    let mut res: bool;
    sb = x.is_sign_negative();
    inf = x.is_infinite();
    nan = x.is_nan();
    sb_or_inf = (sb || inf);
    bad = (sb_or_inf || nan);
    ff = false;
    if bad {
        return ff;
    }
    pow56f = 72057594037927936f64;
    ge = (x >= pow56f);
    if ge {
        return ff;
    }
    i = (x as u64);
    back = (i as f64);
    eqv = (back == x);
    pow56 = 72057594037927936u64;
    lt = (i < pow56);
    res = (eqv && lt);
    return res;
}

fn float_to_index(x: f64) -> u64 {
    let mut simple: bool;
    let mut si: u64;
    let mut bits: u64;
    let mut c52: u64;
    let mut shr: u64;
    let mut c2047: u64;
    let mut biased_exp: u64;
    let mut mmask: u64;
    let mut mantissa: u64;
    let mut be_i: i64;
    let mut c1023i: i64;
    let mut unbiased_exp: i64;
    let mut mantissa_enc: u64;
    let mut exp_enc: u64;
    let mut bit63: u64;
    let mut expsh: u64;
    let mut hi: u64;
    let mut r: u64;
    simple = is_simple_float(x);
    if simple {
        si = (x as u64);
        return si;
    }
    bits = x.to_bits();
    c52 = 52u64;
    shr = (bits >> c52);
    c2047 = 2047u64;
    biased_exp = (shr & c2047);
    mmask = 4503599627370495u64;
    mantissa = (bits & mmask);
    be_i = (biased_exp as i64);
    c1023i = 1023i64;
    unbiased_exp = (be_i - c1023i);
    mantissa_enc = update_mantissa(unbiased_exp, mantissa);
    exp_enc = encode_exponent(biased_exp);
    bit63 = 9223372036854775808u64;
    expsh = (exp_enc << c52);
    hi = (bit63 | expsh);
    r = (hi | mantissa_enc);
    return r;
}

fn index_to_float(i: u64) -> f64 {
    let mut c63: u64;
    let mut top: u64;
    let mut c0: u64;
    let mut isint: bool;
    let mut m56: u64;
    let mut integral: u64;
    let mut f: f64;
    let mut c52: u64;
    let mut shr: u64;
    let mut c2047: u64;
    let mut exp_enc: u64;
    let mut biased_exp: u64;
    let mut mmask: u64;
    let mut mantissa_enc: u64;
    let mut be_i: i64;
    let mut c1023i: i64;
    let mut unbiased_exp: i64;
    let mut mantissa: u64;
    let mut besh: u64;
    let mut bitsr: u64;
    let mut f2: f64;
    c63 = 63u64;
    top = (i >> c63);
    c0 = 0u64;
    isint = (top == c0);
    if isint {
        m56 = 72057594037927935u64;
        integral = (i & m56);
        f = (integral as f64);
        return f;
    }
    c52 = 52u64;
    shr = (i >> c52);
    c2047 = 2047u64;
    exp_enc = (shr & c2047);
    biased_exp = decode_exponent(exp_enc);
    mmask = 4503599627370495u64;
    mantissa_enc = (i & mmask);
    be_i = (biased_exp as i64);
    c1023i = 1023i64;
    unbiased_exp = (be_i - c1023i);
    mantissa = update_mantissa(unbiased_exp, mantissa_enc);
    besh = (biased_exp << c52);
    bitsr = (besh | mantissa);
    f2 = f64::from_bits(bitsr);
    return f2;
}

fn min_reversed_in_range(lo: u64, hi: u64, n: i64) -> u64 {
    let mut zero_i: i64;
    let mut isz: bool;
    let mut c0: u64;
    let mut one: u64;
    let mut lop1: u64;
    let mut k_lo: u64;
    let mut k_hi: u64;
    let mut le: bool;
    let mut onei: i64;
    let mut nm1: i64;
    let mut rec: u64;
    let mut two: u64;
    let mut r: u64;
    zero_i = 0i64;
    isz = (n == zero_i);
    c0 = 0u64;
    if isz {
        return c0;
    }
    one = 1u64;
    lop1 = (lo + one);
    k_lo = (lop1 >> one);
    k_hi = (hi >> one);
    le = (k_lo <= k_hi);
    if le {
        onei = 1i64;
        nm1 = (n - onei);
        rec = min_reversed_in_range(k_lo, k_hi, nm1);
        two = 2u64;
        r = (rec * two);
        return r;
    }
    return lo;
}

fn simplest_in_range(lo: f64, hi: f64) -> f64 {
    let mut c: f64;
    let mut cle: bool;
    let mut pow56f: f64;
    let mut clt: bool;
    let mut inrange: bool;
    let mut lo_bits: u64;
    let mut hi_bits: u64;
    let mut c52: u64;
    let mut e_lo: u64;
    let mut e_hi: u64;
    let mut mmask: u64;
    let mut m_lo: u64;
    let mut m_hi: u64;
    let mut c1023: u64;
    let mut cond: bool;
    let mut ncond: bool;
    let mut e: u64;
    let mut m_min: u64;
    let mut m_max: u64;
    let mut eq_hilo: bool;
    let mut ne_hilo: bool;
    let mut eq_lohi: bool;
    let mut ne_lohi: bool;
    let mut zero: u64;
    let mut c1023i: i64;
    let mut e_i: i64;
    let mut unbiased: i64;
    let mut c52i: i64;
    let mut ge52: bool;
    let mut esh0: u64;
    let mut bitsr0: u64;
    let mut f0: f64;
    let mut zero_i: i64;
    let mut le0: bool;
    let mut n_frac: i64;
    let mut nle0: bool;
    let mut sub: i64;
    let mut one: u64;
    let mut shl1: u64;
    let mut low_mask: u64;
    let mut h: u64;
    let mut l_lo: u64;
    let mut l_hi: u64;
    let mut minrev: u64;
    let mut hsh: u64;
    let mut m_best: u64;
    let mut esh: u64;
    let mut bitsr: u64;
    let mut f: f64;
    c = lo.ceil();
    cle = (c <= hi);
    pow56f = 72057594037927936f64;
    clt = (c < pow56f);
    inrange = (cle && clt);
    if inrange {
        return c;
    }
    lo_bits = lo.to_bits();
    hi_bits = hi.to_bits();
    c52 = 52u64;
    e_lo = (lo_bits >> c52);
    e_hi = (hi_bits >> c52);
    mmask = 4503599627370495u64;
    m_lo = (lo_bits & mmask);
    m_hi = (hi_bits & mmask);
    c1023 = 1023u64;
    cond = (e_lo >= c1023);
    ncond = (!cond);
    e = e_lo;
    m_min = m_lo;
    m_max = m_hi;
    eq_hilo = (e_hi == e_lo);
    ne_hilo = (!eq_hilo);
    if ne_hilo {
        m_max = mmask;
    }
    if ncond {
        e = e_hi;
        m_max = m_hi;
        eq_lohi = (e_lo == e_hi);
        ne_lohi = (!eq_lohi);
        if ne_lohi {
            zero = 0u64;
            m_min = zero;
        }
    }
    c1023i = 1023i64;
    e_i = (e as i64);
    unbiased = (e_i - c1023i);
    c52i = 52i64;
    ge52 = (unbiased >= c52i);
    if ge52 {
        esh0 = (e << c52);
        bitsr0 = (esh0 | m_min);
        f0 = f64::from_bits(bitsr0);
        return f0;
    }
    zero_i = 0i64;
    le0 = (unbiased <= zero_i);
    n_frac = c52i;
    nle0 = (!le0);
    if nle0 {
        sub = (c52i - unbiased);
        n_frac = sub;
    }
    one = 1u64;
    shl1 = (one << n_frac);
    low_mask = (shl1 - one);
    h = (m_min >> n_frac);
    l_lo = (m_min & low_mask);
    l_hi = (m_max & low_mask);
    minrev = min_reversed_in_range(l_lo, l_hi, n_frac);
    hsh = (h << n_frac);
    m_best = (hsh | minrev);
    esh = (e << c52);
    bitsr = (esh | m_best);
    f = f64::from_bits(bitsr);
    return f;
}
