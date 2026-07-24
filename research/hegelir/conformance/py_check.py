#!/usr/bin/env python3
# Check the EMITTED Python module against the Rust reference golden vectors.
import sys
import float_index_gen as g

total = 0
mism = 0
by = {}
fails = []
with open(sys.argv[1]) as f:
    for line in f:
        line = line.rstrip("\n")
        if not line:
            continue
        p = line.split("\t")
        kind = p[0]
        if kind == "F2I":
            inp, want = int(p[1], 16), int(p[2], 16)
            got = g.float_to_index(g.bits2f(inp))
        elif kind == "I2F":
            inp, want = int(p[1], 16), int(p[2], 16)
            got = g.f2bits(g.index_to_float(inp))
        elif kind == "SIR":
            lo, hi, want = int(p[1], 16), int(p[2], 16), int(p[3], 16)
            got = g.f2bits(g.simplest_in_range(g.bits2f(lo), g.bits2f(hi)))
        else:
            raise SystemExit("unknown kind " + kind)
        total += 1
        t, mm = by.get(kind, (0, 0))
        if got != want:
            mism += 1
            mm += 1
            if len(fails) < 10:
                fails.append(f"  {kind} in={p[1]} got={got:016x} want={want:016x}")
        by[kind] = (t + 1, mm)

print(f"checked {total} vectors across {len(by)} kinds")
for k in ("F2I", "I2F", "SIR"):
    if k in by:
        print(f"  {k:<4} {by[k][0]:6d} checked  {by[k][1]:6d} mismatched")
if mism == 0:
    print("RESULT: PASS — emitted Python is bit-exact with the Rust reference")
else:
    print(f"RESULT: FAIL — {mism}/{total} mismatched")
    for s in fails:
        print(s)
    sys.exit(1)
