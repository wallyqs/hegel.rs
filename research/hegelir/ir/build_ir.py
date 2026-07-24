#!/usr/bin/env python3
# Author the HegelIR module for the float_index functional core.
#
# This is a compact, purpose-built IR (the "build our own, Ax as blueprint"
# path): fixed-width unsigned/signed ints, floats, bit ops, and float<->bits
# reinterpretation — exactly the vocabulary AxIR's Core lacks. One IR module,
# lowered by emit.go to native Go and Python.
import json

# expression constructors
def c(t, v):  return ["c", t, str(v)]
def v(n):     return ["v", n]
def call(f, *a): return ["call", f, list(a)]
def u(op, x): return ["u", op, x]
def b(op, x, y): return ["b", op, x, y]
def sel(cond, a, d): return ["sel", cond, a, d]

# statement constructors
def lett(n, e): return ["let", n, e]
def ret(e):     return ["ret", e]
def iff(cond, then, els=None): return ["if", cond, then, els or []]

U64_MASK52 = 4503599627370495          # (1<<52)-1
BIT63      = 9223372036854775808       # 1<<63
POW56      = 72057594037927936         # 1<<56
POW56M1    = 72057594037927935         # (1<<56)-1

module = {
  "types": ["u64", "i64", "f64", "bool"],
  "functions": [
    {"name": "encode_exponent", "params": [["biased_exp", "u64"]], "ret": "u64", "body": [
      iff(b("eq", v("biased_exp"), c("u64", 2047)), [ret(c("u64", 2047))]),
      iff(b("ge", v("biased_exp"), c("u64", 1023)),
          [ret(b("sub", v("biased_exp"), c("u64", 1023)))],
          [ret(b("sub", c("u64", 2046), v("biased_exp")))]),
    ]},
    {"name": "decode_exponent", "params": [["enc", "u64"]], "ret": "u64", "body": [
      iff(b("eq", v("enc"), c("u64", 2047)), [ret(c("u64", 2047))]),
      iff(b("le", v("enc"), c("u64", 1023)),
          [ret(b("add", v("enc"), c("u64", 1023)))],
          [ret(b("sub", c("u64", 2046), v("enc")))]),
    ]},
    {"name": "reverse_bits_n", "params": [["val", "u64"], ["n", "u64"]], "ret": "u64", "body": [
      iff(b("eq", v("n"), c("u64", 0)), [ret(c("u64", 0))]),
      ret(b("shr", u("revbits", v("val")), b("sub", c("u64", 64), v("n")))),
    ]},
    {"name": "update_mantissa", "params": [["unbiased_exp", "i64"], ["mantissa", "u64"]], "ret": "u64", "body": [
      iff(b("le", v("unbiased_exp"), c("i64", 0)),
          [ret(call("reverse_bits_n", v("mantissa"), c("u64", 52)))],
          [iff(b("le", v("unbiased_exp"), c("i64", 51)),
              [lett("n_frac", u("i2u", b("sub", c("i64", 52), v("unbiased_exp")))),
               lett("frac_mask", b("sub", b("shl", c("u64", 1), v("n_frac")), c("u64", 1))),
               lett("frac", b("and", v("mantissa"), v("frac_mask"))),
               ret(b("or", b("xor", v("mantissa"), v("frac")),
                          call("reverse_bits_n", v("frac"), v("n_frac"))))],
              [ret(v("mantissa"))])]),
    ]},
    {"name": "is_simple_float", "params": [["x", "f64"]], "ret": "bool", "body": [
      iff(b("lor", b("lor", u("signbit", v("x")), u("isinf", v("x"))), u("isnan", v("x"))),
          [ret(c("bool", "false"))]),
      iff(b("ge", v("x"), c("f64", POW56)), [ret(c("bool", "false"))]),
      lett("i", u("f2u", v("x"))),
      ret(b("land", b("eq", u("u2f", v("i")), v("x")), b("lt", v("i"), c("u64", POW56)))),
    ]},
    {"name": "float_to_index", "params": [["x", "f64"]], "ret": "u64", "body": [
      iff(call("is_simple_float", v("x")), [ret(u("f2u", v("x")))]),
      lett("bits", u("f64bits", v("x"))),
      lett("biased_exp", b("and", b("shr", v("bits"), c("u64", 52)), c("u64", 2047))),
      lett("mantissa", b("and", v("bits"), c("u64", U64_MASK52))),
      lett("unbiased_exp", b("sub", u("u2i", v("biased_exp")), c("i64", 1023))),
      lett("mantissa_enc", call("update_mantissa", v("unbiased_exp"), v("mantissa"))),
      lett("exp_enc", call("encode_exponent", v("biased_exp"))),
      ret(b("or", b("or", c("u64", BIT63), b("shl", v("exp_enc"), c("u64", 52))), v("mantissa_enc"))),
    ]},
    {"name": "index_to_float", "params": [["i", "u64"]], "ret": "f64", "body": [
      iff(b("eq", b("shr", v("i"), c("u64", 63)), c("u64", 0)),
          [lett("integral", b("and", v("i"), c("u64", POW56M1))),
           ret(u("u2f", v("integral")))]),
      lett("exp_enc", b("and", b("shr", v("i"), c("u64", 52)), c("u64", 2047))),
      lett("biased_exp", call("decode_exponent", v("exp_enc"))),
      lett("mantissa_enc", b("and", v("i"), c("u64", U64_MASK52))),
      lett("unbiased_exp", b("sub", u("u2i", v("biased_exp")), c("i64", 1023))),
      lett("mantissa", call("update_mantissa", v("unbiased_exp"), v("mantissa_enc"))),
      ret(u("bitsf64", b("or", b("shl", v("biased_exp"), c("u64", 52)), v("mantissa")))),
    ]},
    {"name": "min_reversed_in_range", "params": [["lo", "u64"], ["hi", "u64"], ["n", "i64"]], "ret": "u64", "body": [
      iff(b("eq", v("n"), c("i64", 0)), [ret(c("u64", 0))]),
      lett("k_lo", b("shr", b("add", v("lo"), c("u64", 1)), c("u64", 1))),
      lett("k_hi", b("shr", v("hi"), c("u64", 1))),
      iff(b("le", v("k_lo"), v("k_hi")),
          [ret(b("mul", call("min_reversed_in_range", v("k_lo"), v("k_hi"), b("sub", v("n"), c("i64", 1))), c("u64", 2)))],
          [ret(v("lo"))]),
    ]},
    {"name": "simplest_in_range", "params": [["lo", "f64"], ["hi", "f64"]], "ret": "f64", "body": [
      lett("c", u("ceil", v("lo"))),
      iff(b("land", b("le", v("c"), v("hi")), b("lt", v("c"), c("f64", POW56))),
          [ret(v("c"))]),
      lett("lo_bits", u("f64bits", v("lo"))),
      lett("hi_bits", u("f64bits", v("hi"))),
      lett("e_lo", b("shr", v("lo_bits"), c("u64", 52))),
      lett("e_hi", b("shr", v("hi_bits"), c("u64", 52))),
      lett("m_lo", b("and", v("lo_bits"), c("u64", U64_MASK52))),
      lett("m_hi", b("and", v("hi_bits"), c("u64", U64_MASK52))),
      lett("cond", b("ge", v("e_lo"), c("u64", 1023))),
      lett("e", sel(v("cond"), v("e_lo"), v("e_hi"))),
      lett("m_min", sel(v("cond"), v("m_lo"), sel(b("eq", v("e_lo"), v("e_hi")), v("m_lo"), c("u64", 0)))),
      lett("m_max", sel(v("cond"), sel(b("eq", v("e_hi"), v("e_lo")), v("m_hi"), c("u64", U64_MASK52)), v("m_hi"))),
      lett("unbiased", b("sub", u("u2i", v("e")), c("i64", 1023))),
      iff(b("ge", v("unbiased"), c("i64", 52)),
          [ret(u("bitsf64", b("or", b("shl", v("e"), c("u64", 52)), v("m_min"))))],
          [lett("n_frac", sel(b("le", v("unbiased"), c("i64", 0)), c("i64", 52), b("sub", c("i64", 52), v("unbiased")))),
           lett("low_mask", b("sub", b("shl", c("u64", 1), v("n_frac")), c("u64", 1))),
           lett("h", b("shr", v("m_min"), v("n_frac"))),
           lett("l_lo", b("and", v("m_min"), v("low_mask"))),
           lett("l_hi", b("and", v("m_max"), v("low_mask"))),
           lett("m_best", b("or", b("shl", v("h"), v("n_frac")),
                                  call("min_reversed_in_range", v("l_lo"), v("l_hi"), v("n_frac")))),
           ret(u("bitsf64", b("or", b("shl", v("e"), c("u64", 52)), v("m_best"))))]),
    ]},
  ],
}

with open("float_index.ir.json", "w") as f:
    json.dump(module, f, indent=1)
print("wrote float_index.ir.json with", len(module["functions"]), "functions")
