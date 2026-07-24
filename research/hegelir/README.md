# HegelIR — research spike

A feasibility spike for defining Hegel's deterministic **functional core** in a
single intermediate representation and compiling it into **native, idiomatic
code in multiple languages** — instead of shipping one `libhegel` binary that
every language binds to over the C ABI.

This is exploratory research, not part of the shipped crates.

## Why

Today every language binding links the native `libhegel` engine through the C
ABI. An alternative — inspired by [Ax](https://github.com/ax-llm/ax), whose
`AxIR` compiles one semantic model into checked-in native packages for Go,
Python, Java, C++, and Rust — is to generate a native engine per language from a
single source of truth. That would give: no per-platform native binary, native
debuggable code, guaranteed determinism, and one definition instead of N
hand-synced bindings.

Ax's *architecture* (dialects → a low-level Core → per-language emitters → a
golden-fixture `verify` gate) is a strong blueprint, and its compiler is written
in Go. But its Core IR is built for LLM orchestration: its type system is
`string/bool/int/i64/f64/bytes` with no unsigned/fixed-width integers, and it
has **no bitwise or float-bits intrinsics**. Hegel's core is fundamentally
`u64` bit-twiddling and `f64`↔bits reinterpretation, so AxIR cannot express it
as-is.

So we **adopt AxIR's textual Core format and extend it** with just the
vocabulary we need. The IR module (`ir/float_index.axir`) is written in the same
MLIR-like syntax Ax uses — `module`/`op core.func @name`/`body @entry(%p: T)`,
flat-SSA Core bodies where every operand is a `%binding`, `core.const` /
`core.call @fn(...)` / `core.call intrinsic.x(...)` / `core.if` / `core.return`
— plus our additions: the `u64` type and the `intrinsic.bit.*`,
`intrinsic.float.*`, and `intrinsic.cast.*` intrinsic families.

## What the spike proves

The deterministic choice→value encoding layer — the part cross-language
reproducibility actually depends on (the seed/RNG stream is deliberately *not*
stable even across Rust builds; the reproduce **blob** is what carries
determinism) — can be reimplemented as native Go, Python, and Rust that is
**bit-exact** with the Rust engine, driven entirely by one IR.

The component chosen is `core/float_index.rs` (Hypothesis's float lexicographic
encoding: `float_to_index` / `index_to_float` / `simplest_in_range`) — pure,
dependency-free, gnarly bit manipulation, and determinism-critical for float
shrinking.

Latest run: **10,050 golden vectors** (curated edge cases + ~6,000 randomized
arbitrary bit patterns), **0 mismatches** across all three emitted targets — Go,
Python, and Rust. The Rust target closes the loop: because the reference *is* the
engine's own Rust, an emitted-Rust match is direct evidence that the IR
faithfully reproduces the original implementation.

## Layout

| Path | Role |
|------|------|
| `reference/ref.rs` | Rust reference: the `float_index` functions copied verbatim from `hegel-c/src/native/core/float_index.rs`, plus a golden-vector generator. The source of truth. |
| `ir/float_index.axir` | **HegelIR**: the functional core expressed in AxIR-style text (adopted from Ax, extended with `u64` + `intrinsic.bit.*`/`float.*`/`cast.*`). The source of truth for codegen. |
| `emitter/emit.go` | The emitter (à la AxIR's Go compiler): tokenizes + parses the `.axir` module, validates it, sanitizes identifiers, and lowers it to native Go, Python, or Rust, encoding each language's integer/float semantics. |
| `targets/go/float_index_gen.go` | Generated Go (checked in, like Ax's `packages/<lang>`). Do not edit by hand — re-run `run.sh`. |
| `targets/python/float_index_gen.py` | Generated Python (checked in). Do not edit by hand. |
| `targets/rust/float_index_gen.rs` | Generated Rust (checked in). Do not edit by hand. |
| `conformance/checker.go` | Go conformance checker: runs the vectors through the emitted Go. |
| `conformance/py_check.py` | Python conformance checker: runs the vectors through the emitted Python. |
| `conformance/checker.rs` | Rust conformance checker: `include!`s the emitted Rust and runs the vectors through it. |
| `spike1/floatindex.go` | The earlier milestone: a hand-written Go port (before the emitter existed), used to prove bit-exactness was achievable at all. |
| `run.sh` | End-to-end: reference → vectors → emit → check all three targets. |

## Run

```bash
research/hegelir/run.sh
```

Needs `rustc`, `go`, and `python3` on PATH.

## The key cross-language hazard (handled once, in the emitter)

Languages disagree on the primitives this code is built from. Example: an
out-of-range `float64`→`uint64` cast is *saturating* in Rust (`1e300 as u64 ==
u64::MAX`) but *implementation-defined* in Go (`uint64(1e300) == 2^63`). The
emitter encodes each language's rules — e.g. Python masks every `u64` op with
`& MASK` and uses `struct` for the float bitcast — so the single IR stays
bit-exact everywhere. The golden-vector gate is what keeps it honest.

## Scope / limits

- The IR covers what this one component needs. The full core also needs
  records/structs, collections, the bignum path, and Unicode tables — more IR
  vocabulary, all mechanical.
- Three targets (Go, Python, Rust). Java/C++ are additional emitters.
- The emitter **validates** the IR before emit (unknown intrinsics, wrong
  arity, undefined bindings, invalid const types) and **sanitizes** identifiers
  that collide with any target's keywords (e.g. `%false` → `false_`). It is
  still arity/definedness checking, not full type inference, and the IR
  vocabulary is scalar-only — no loops or aggregates yet.
- This is the deterministic encoding/replay path only. The effectful shell
  (RNG, on-disk database, panic/exception mapping) stays a thin per-language
  layer — the same split the C ABI already draws.

## Possible next steps

1. Widen the IR to the blob/base64 codec — the actual cross-language wire format
   (needs loops + byte-aggregate vocabulary; the real generalization test).
2. Extend the validator from arity/definedness to full type inference/checking.
3. Add Java / C++ targets to match Ax's full backend set.
4. Write up a full design doc (IR spec, core/shell split, conformance-suite plan).
