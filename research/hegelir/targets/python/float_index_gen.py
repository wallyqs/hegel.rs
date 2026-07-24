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
    if (biased_exp == 2047):
        return 2047
    if (biased_exp >= 1023):
        return ((biased_exp - 1023) & MASK)
    return ((2046 - biased_exp) & MASK)

def decode_exponent(enc):
    if (enc == 2047):
        return 2047
    if (enc <= 1023):
        return ((enc + 1023) & MASK)
    return ((2046 - enc) & MASK)

def reverse_bits_n(val, n):
    if (n == 0):
        return 0
    return (rev64(val) >> ((64 - n) & MASK))

def update_mantissa(unbiased_exp, mantissa):
    if (unbiased_exp <= 0):
        return reverse_bits_n(mantissa, 52)
    if (unbiased_exp <= 51):
        n_frac = ((52 - unbiased_exp) & MASK)
        frac_mask = ((((1 << n_frac) & MASK) - 1) & MASK)
        frac = (mantissa & frac_mask)
        return ((mantissa ^ frac) | reverse_bits_n(frac, n_frac))
    return mantissa

def is_simple_float(x):
    if (((math.copysign(1.0, x) < 0.0) or math.isinf(x)) or math.isnan(x)):
        return False
    if (x >= float(72057594037927936)):
        return False
    i = int(x)
    return ((float(i) == x) and (i < 72057594037927936))

def float_to_index(x):
    if is_simple_float(x):
        return int(x)
    bits = f2bits(x)
    biased_exp = ((bits >> 52) & 2047)
    mantissa = (bits & 4503599627370495)
    unbiased_exp = ((biased_exp) - 1023)
    mantissa_enc = update_mantissa(unbiased_exp, mantissa)
    exp_enc = encode_exponent(biased_exp)
    return ((9223372036854775808 | ((exp_enc << 52) & MASK)) | mantissa_enc)

def index_to_float(i):
    if ((i >> 63) == 0):
        integral = (i & 72057594037927935)
        return float(integral)
    exp_enc = ((i >> 52) & 2047)
    biased_exp = decode_exponent(exp_enc)
    mantissa_enc = (i & 4503599627370495)
    unbiased_exp = ((biased_exp) - 1023)
    mantissa = update_mantissa(unbiased_exp, mantissa_enc)
    return bits2f((((biased_exp << 52) & MASK) | mantissa))

def min_reversed_in_range(lo, hi, n):
    if (n == 0):
        return 0
    k_lo = (((lo + 1) & MASK) >> 1)
    k_hi = (hi >> 1)
    if (k_lo <= k_hi):
        return ((min_reversed_in_range(k_lo, k_hi, (n - 1)) * 2) & MASK)
    return lo

def simplest_in_range(lo, hi):
    c = float(math.ceil(lo))
    if ((c <= hi) and (c < float(72057594037927936))):
        return c
    lo_bits = f2bits(lo)
    hi_bits = f2bits(hi)
    e_lo = (lo_bits >> 52)
    e_hi = (hi_bits >> 52)
    m_lo = (lo_bits & 4503599627370495)
    m_hi = (hi_bits & 4503599627370495)
    cond = (e_lo >= 1023)
    e = (e_lo if cond else e_hi)
    m_min = (m_lo if cond else (m_lo if (e_lo == e_hi) else 0))
    m_max = ((m_hi if (e_hi == e_lo) else 4503599627370495) if cond else m_hi)
    unbiased = ((e) - 1023)
    if (unbiased >= 52):
        return bits2f((((e << 52) & MASK) | m_min))
    n_frac = (52 if (unbiased <= 0) else (52 - unbiased))
    low_mask = ((((1 << n_frac) & MASK) - 1) & MASK)
    h = (m_min >> n_frac)
    l_lo = (m_min & low_mask)
    l_hi = (m_max & low_mask)
    m_best = (((h << n_frac) & MASK) | min_reversed_in_range(l_lo, l_hi, n_frac))
    return bits2f((((e << 52) & MASK) | m_best))
