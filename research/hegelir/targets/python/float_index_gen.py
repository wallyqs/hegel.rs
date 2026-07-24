import math, struct
MASK = (1 << 64) - 1
def rev64(x):
    r = 0
    for _ in range(64):
        r = ((r << 1) | (x & 1)) & MASK
        x >>= 1
    return r
def f2bits(x): return struct.unpack('<Q', struct.pack('<d', x))[0]
def bits2f(x): return struct.unpack('<d', struct.pack('<Q', x))[0]

def encode_exponent(biased_exp):
    c2047 = 2047
    is2047 = (biased_exp == c2047)
    if is2047:
        return c2047
    c1023 = 1023
    ge = (biased_exp >= c1023)
    if ge:
        r = ((biased_exp - c1023) & MASK)
        return r
    c2046 = 2046
    r2 = ((c2046 - biased_exp) & MASK)
    return r2

def decode_exponent(enc):
    c2047 = 2047
    is2047 = (enc == c2047)
    if is2047:
        return c2047
    c1023 = 1023
    le = (enc <= c1023)
    if le:
        r = ((enc + c1023) & MASK)
        return r
    c2046 = 2046
    r2 = ((c2046 - enc) & MASK)
    return r2

def reverse_bits_n(val, n):
    c0 = 0
    isz = (n == c0)
    if isz:
        return c0
    c64 = 64
    rev = rev64(val)
    sh = ((c64 - n) & MASK)
    r = (rev >> sh)
    return r

def update_mantissa(unbiased_exp, mantissa):
    zero_i = 0
    le0 = (unbiased_exp <= zero_i)
    if le0:
        c52 = 52
        r = reverse_bits_n(mantissa, c52)
        return r
    c51i = 51
    le51 = (unbiased_exp <= c51i)
    if le51:
        c52i = 52
        nfi = (c52i - unbiased_exp)
        n_frac = (nfi & MASK)
        one = 1
        shifted = ((one << n_frac) & MASK)
        frac_mask = ((shifted - one) & MASK)
        frac = (mantissa & frac_mask)
        xored = (mantissa ^ frac)
        revfrac = reverse_bits_n(frac, n_frac)
        r = (xored | revfrac)
        return r
    return mantissa

def is_simple_float(x):
    sb = (math.copysign(1.0, x) < 0.0)
    inf = math.isinf(x)
    nan = math.isnan(x)
    sb_or_inf = (sb or inf)
    bad = (sb_or_inf or nan)
    false = False
    if bad:
        return false
    pow56f = float(72057594037927936)
    ge = (x >= pow56f)
    if ge:
        return false
    i = int(x)
    back = float(i)
    eqv = (back == x)
    pow56 = 72057594037927936
    lt = (i < pow56)
    res = (eqv and lt)
    return res

def float_to_index(x):
    simple = is_simple_float(x)
    if simple:
        si = int(x)
        return si
    bits = f2bits(x)
    c52 = 52
    shr = (bits >> c52)
    c2047 = 2047
    biased_exp = (shr & c2047)
    mmask = 4503599627370495
    mantissa = (bits & mmask)
    be_i = (biased_exp)
    c1023i = 1023
    unbiased_exp = (be_i - c1023i)
    mantissa_enc = update_mantissa(unbiased_exp, mantissa)
    exp_enc = encode_exponent(biased_exp)
    bit63 = 9223372036854775808
    expsh = ((exp_enc << c52) & MASK)
    hi = (bit63 | expsh)
    r = (hi | mantissa_enc)
    return r

def index_to_float(i):
    c63 = 63
    top = (i >> c63)
    c0 = 0
    isint = (top == c0)
    if isint:
        m56 = 72057594037927935
        integral = (i & m56)
        f = float(integral)
        return f
    c52 = 52
    shr = (i >> c52)
    c2047 = 2047
    exp_enc = (shr & c2047)
    biased_exp = decode_exponent(exp_enc)
    mmask = 4503599627370495
    mantissa_enc = (i & mmask)
    be_i = (biased_exp)
    c1023i = 1023
    unbiased_exp = (be_i - c1023i)
    mantissa = update_mantissa(unbiased_exp, mantissa_enc)
    besh = ((biased_exp << c52) & MASK)
    bitsr = (besh | mantissa)
    f2 = bits2f(bitsr)
    return f2

def min_reversed_in_range(lo, hi, n):
    zero_i = 0
    isz = (n == zero_i)
    c0 = 0
    if isz:
        return c0
    one = 1
    lop1 = ((lo + one) & MASK)
    k_lo = (lop1 >> one)
    k_hi = (hi >> one)
    le = (k_lo <= k_hi)
    if le:
        onei = 1
        nm1 = (n - onei)
        rec = min_reversed_in_range(k_lo, k_hi, nm1)
        two = 2
        r = ((rec * two) & MASK)
        return r
    return lo

def simplest_in_range(lo, hi):
    c = float(math.ceil(lo))
    cle = (c <= hi)
    pow56f = float(72057594037927936)
    clt = (c < pow56f)
    inrange = (cle and clt)
    if inrange:
        return c
    lo_bits = f2bits(lo)
    hi_bits = f2bits(hi)
    c52 = 52
    e_lo = (lo_bits >> c52)
    e_hi = (hi_bits >> c52)
    mmask = 4503599627370495
    m_lo = (lo_bits & mmask)
    m_hi = (hi_bits & mmask)
    c1023 = 1023
    cond = (e_lo >= c1023)
    ncond = (not cond)
    e = e_lo
    m_min = m_lo
    m_max = m_hi
    eq_hilo = (e_hi == e_lo)
    ne_hilo = (not eq_hilo)
    if ne_hilo:
        m_max = mmask
    if ncond:
        e = e_hi
        m_max = m_hi
        eq_lohi = (e_lo == e_hi)
        ne_lohi = (not eq_lohi)
        if ne_lohi:
            zero = 0
            m_min = zero
    c1023i = 1023
    e_i = (e)
    unbiased = (e_i - c1023i)
    c52i = 52
    ge52 = (unbiased >= c52i)
    if ge52:
        esh0 = ((e << c52) & MASK)
        bitsr0 = (esh0 | m_min)
        f0 = bits2f(bitsr0)
        return f0
    zero_i = 0
    le0 = (unbiased <= zero_i)
    n_frac = c52i
    nle0 = (not le0)
    if nle0:
        sub = (c52i - unbiased)
        n_frac = sub
    one = 1
    shl1 = ((one << n_frac) & MASK)
    low_mask = ((shl1 - one) & MASK)
    h = (m_min >> n_frac)
    l_lo = (m_min & low_mask)
    l_hi = (m_max & low_mask)
    minrev = min_reversed_in_range(l_lo, l_hi, n_frac)
    hsh = ((h << n_frac) & MASK)
    m_best = (hsh | minrev)
    esh = ((e << c52) & MASK)
    bitsr = (esh | m_best)
    f = bits2f(bitsr)
    return f
