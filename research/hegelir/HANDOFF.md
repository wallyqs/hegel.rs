# HegelIR — Handoff & Plan

You are picking up a research spike that lives entirely in `research/hegelir/`.
Read `README.md` first (what/why), then this file (how it's built and what to do
next). Nothing here touches the shipped crates.

## 0. TL;DR status

- **Goal:** define Hegel's deterministic *functional core* in one IR and compile
  it to native, idiomatic code in many languages — replacing "ship one
  `libhegel` binary + FFI from every language."
- **Done:** one AxIR-style IR module (`ir/float_index.axir`) → an emitter
  (`emitter/emit.go`) that validates + sanitizes + lowers it to native **Go,
  Python, and Rust**, all **bit-exact** with the engine's own Rust across 10,050
  golden vectors. `run.sh` does the whole loop end-to-end.
- **Component covered:** `hegel-c/src/native/core/float_index.rs` (float
  lexicographic encoding). Scalar/bitwise only.
- **Next big milestone:** the base64 codec — forces the IR to grow **loops +
  byte aggregates**, and is the actual cross-language wire format (blobs).

Verify everything works before changing anything:

```bash
research/hegelir/run.sh   # needs rustc, go, python3
```

Expect three `RESULT: PASS` lines + one "malformed IR rejected".

## 1. Mental model (the load-bearing ideas)

- **Two layers.** The engine splits into a *pure deterministic core* (choice
  sequences, the data tree, shrink ordering, targeting, RNG, regex gen, interval
  sets, blob encoding) and an *effectful shell* (on-disk example DB, threading,
  turning failures into each language's panic/exception). Only the **pure core**
  is a codegen target. The shell stays a thin hand-written per-language layer —
  the same seam the C ABI already draws.
- **Determinism travels in the blob, not the seed.** `hegel-c/src/native/rng.rs`
  uses `rand::SmallRng`, whose stream is explicitly *not* stable across builds or
  architectures. Cross-language reproducibility comes from replaying the
  **reproduce blob** (the choice sequence) through the deterministic
  choice→value encoding. So the IR must reproduce that *encoding* bit-for-bit;
  it does **not** need to reproduce the RNG stream. This is why base64/blob (the
  wire format) matters more than porting the RNG.
- **Why our own IR, not AxIR directly.** Ax (https://github.com/ax-llm/ax) is the
  design blueprint: MLIR-like dialects → a low-level Core → per-language Go
  emitters → a golden-fixture `verify` gate (its compiler is Go, under
  `tools/axir/internal/axir/`). But AxIR's Core is built for LLM orchestration —
  types are `string/bool/int/i64/f64/bytes`, with **no unsigned/fixed-width ints
  and no bitwise/float-bits intrinsics** — so it cannot express Hegel's core. We
  **adopt its text syntax and extend it** with `u64` + `intrinsic.bit.*` /
  `intrinsic.float.*` / `intrinsic.cast.*`.
- **The conformance gate is the whole safety story.** Every target must
  reproduce golden vectors emitted from the Rust reference, bit-for-bit. Never
  weaken this. Randomized inputs (arbitrary bit patterns) matter — they catch the
  cross-language primitive disagreements (see §6).

## 2. Inventory

| Path | Role |
|------|------|
| `reference/ref.rs` | Rust reference: engine functions copied **verbatim** from `hegel-c/src/native/core/float_index.rs` + a golden-vector generator. Source of truth. |
| `ir/float_index.axir` | The IR module (AxIR-style text). |
| `emitter/emit.go` | Parser + validator + sanitizer + Go/Python/Rust emitters. |
| `targets/{go,python,rust}/float_index_gen.*` | **Generated** (checked in, like Ax's `packages/<lang>`). Never hand-edit — `run.sh` regenerates them. |
| `conformance/checker.go`, `py_check.py`, `checker.rs` | Per-language vector checkers. |
| `spike1/floatindex.go` | The pre-emitter hand-port milestone (historical). |
| `run.sh` | reference → vectors → emit (3 targets) → check → validator negative test. |

**Invariants to preserve**
1. Generated files under `targets/` are outputs — regenerate via `run.sh`, never
   edit by hand; commit them (checked-in generated code is intentional).
2. `reference/ref.rs` functions must stay **verbatim** copies of the engine
   source (only the internal debug-assert macros are swapped for `debug_assert!`).
   If the engine changes, re-copy.
3. Every target must pass every vector, bit-for-bit. A single mismatch is a bug.

## 3. How the emitter works (so you can extend it)

`emitter/emit.go`, top to bottom:

- **`tokenize`** — chars → tokens (`{}():,=`, `"strings"`, `@names`, `%bindings`,
  idents incl. numbers and dotted names like `intrinsic.bit.shl`).
- **`parser`** (`parseModule`/`parseOp`/`parseBody`/`parseStmts`/`parseExpr`) —
  produces `[]fn`. Skips everything it doesn't model (`type`, `attr`, `tag`,
  module/dialect headers). AST: `fn{name,params,ret,body}`; `stmt` kinds
  `assign|if|return`; `expr` kinds `const|copy|call` (flat SSA — operands are
  binding names or literals, never nested exprs).
- **Typing** — `intrinsicType` (result type of each intrinsic), `exprType`,
  `inferFunc` (builds the type env, the func-scope declaration order for
  pre-declared mutable locals, and the return type). `funcRet` is a global map;
  callers must appear after callees (the IR is ordered topologically).
- **Emitters** — one trio per target: `ri<T>` (renders an `intrinsic.*` call),
  `expr<T>` (renders any expr), `<t>Stmts` (renders statements), `emit<T>`
  (function headers + declarations + body). Go/Rust pre-declare all non-param
  bindings (`var x T` / `let mut x: T;`) then assign; Python just assigns.
- **`sanitizeAST`** — renames any binding/function whose name is in `reserved`
  (Go∪Python∪Rust keywords) to `name+"_"`, uniformly. Run before typing/emit.
- **`validate`** — arity/definedness/known-intrinsic/const-type checks; prints
  `IR error: …` to stderr and exits 1 on any problem.
- **`main`** — read file → tokenize → parse → `sanitizeAST` → `validate` →
  fill `funcRet` → emit for `os.Args[2]` in {`go`,`py`,`rust`}.

### Recipes

- **Add an intrinsic** `intrinsic.foo`: add its result type to `intrinsicType`,
  its arity to `intrinsicArity`, and a rendering to `riGo`/`riPy`/`riRust`.
- **Add a primitive type** `t`: extend `goType`/`rustType`/`validType`, the
  `const` rendering in each `expr<T>`, and `intrinsicType` where relevant.
- **Add a target language**: write `ri<L>`/`expr<L>`/`<l>Stmts`/`emit<L>`, a
  `case` in `main`, a `conformance/checker.<l>`, and wire it into `run.sh`.
  Add its keywords to `reserved`.
- **Add control flow / aggregates** (needed for base64, §5 M1): new `stmt`/`expr`
  kinds in the parser + AST, typing, all three emitters, and the validator.

## 4. Conformance recipe (adding a component)

1. Copy the engine function **verbatim** into `reference/ref.rs`; add a
   generator that prints `KIND\t<hex inputs>\t<hex output>` lines (encode
   floats/bytes as their raw hex so comparison is bit-exact, never decimal).
   Include curated edge cases **and** a few thousand randomized inputs.
2. Author the component in `ir/<name>.axir`.
3. Extend the emitter as needed; regenerate targets.
4. Add the new `KIND` branch to each checker (call the emitted function, compare
   to the expected hex).
5. Wire into `run.sh`. All targets must pass.

## 5. Roadmap (prioritized)

### M1 — base64 codec (the real generalization test) — START HERE
Engine source: `hegel-c/src/native/base64.rs` (standard RFC 4648 alphabet,
padded). Do `base64_encode` first (total; no failure path), then `base64_decode`
(returns `Option<Vec<u8>>` — needs an optional/result return, M1b).

New IR vocabulary required:
- Types: `bytes` (input) and `str` (output/accumulator).
- Const: `core.const bytes "ABC…+/"` (the alphabet; tokenizer already yields the
  quoted token — handle a bytes/string literal in `parseExpr`).
- Expr: `core.strbuf` (new empty builder → type `str`).
- Statements: `core.loop { … }`, `core.break`, `core.append %buf, %val`.
- Intrinsics: `intrinsic.len(%bytes) -> i64`, `intrinsic.bytes.get(%b,%i) -> u64`.

Per-language emission (the important bit — get these exactly right):
| Op | Go | Python | Rust |
|----|----|--------|------|
| `bytes` param | `[]byte` | `bytes` | `&[u8]` |
| `str` local / return | `[]byte` / `string(x)` | `bytearray()` / `x.decode('ascii')` | `Vec<u8>` / `String::from_utf8(x).unwrap()` |
| `core.strbuf` | `[]byte{}` | `bytearray()` | `Vec::new()` |
| `core.append b,v` | `b = append(b, byte(v))` | `b.append(v & 0xFF)` | `b.push(v as u8);` |
| `core.loop` / `core.break` | `for {` / `break` | `while True:` / `break` | `loop {` / `break;` |
| `const bytes "s"` | `[]byte("s")` | `b"s"` | `&b"s"[..]` |
| `intrinsic.len(x)` | `int64(len(x))` | `len(x)` | `(x.len() as i64)` |
| `intrinsic.bytes.get(x,i)` | `uint64(x[i])` | `x[i]` | `(x[i as usize] as u64)` |

Gotchas:
- **Go imports**: base64 uses neither `math` nor `math/bits`. `emitGo` currently
  always emits both imports → unused-import compile error. Make `emitGo` scan
  used intrinsics and import only what's needed (or emit no imports when none).
- Rust bytes const must be sliced (`&b"…"[..]`) to coerce `&[u8; N]` → `&[u8]`.
- Vector kind e.g. `ENC`; generate random byte arrays of length 0..~32 plus the
  length-1/2/3/4 edge cases (padding boundaries).
- M1b `base64_decode` returns optional → design an `optional<T>`/result return
  and emit `([]byte, bool)` (Go), `bytes | None` (Python), `Option<Vec<u8>>`
  (Rust). Validation branches return the "invalid" sentinel.

### M2 — validator → real type inference
Today's `validate` is arity/definedness only. Add operand type-checking (match
each intrinsic's expected operand types against `inferFunc`'s env), throws/effect
discipline once effects exist, and better diagnostics. Ax's
`tools/axir/internal/axir/check_types.go` is the reference implementation.

### M3 — more targets
Add Java and C++ emitters to match Ax's backend set. Each is a `ri`/`expr`/
`stmts`/`emit` trio + checker + `run.sh` wiring + `reserved` additions.

### M4 — broaden the core
Other replay-path components in dependency order: `bignum.rs` (arbitrary-
precision integer draws — two's-complement LE), then `blob.rs` (the reproduce
blob format itself; check whether it uses `miniz_oxide` deflate — compression is
*not* bit-reproducible across languages on the encode side, so only decode may be
portable). Then the typed draws in `hegel-c/src/native/draws/`.

### M5 — write-up / decisions for a human
A design doc (IR spec, the core/shell boundary, emitter architecture, the
conformance-suite plan) and the open questions in §7.

## 6. Cross-language gotchas (why the golden gate is non-negotiable)

- **float→int cast**: out-of-range is *saturating* in Rust (`1e300 as u64 ==
  u64::MAX`) but *implementation-defined* in Go (`uint64(1e300) == 2^63`). The IR
  sidesteps it (`is_simple_float` guards `>= 2^56` before casting); a naive port
  would diverge silently. The randomized vectors catch this class of bug.
- **u64 semantics**: Go/Rust wrap natively; Python ints are unbounded, so the
  Python emitter masks every `u64` add/sub/mul/shl with `& MASK` and does float
  bitcasts via `struct`. Any new u64 op must preserve this.
- **Identifiers**: the sanitizer handles keyword collisions; if you add a target,
  add its keywords to `reserved`.
- **Generated files**: always regenerated by `run.sh`; if a diff shows up in
  `targets/` after an emitter change, that's expected — commit it.

## 7. Open questions for a human

- Is the end goal to **replace** the C ABI/`libhegel` for some languages, or to
  offer generated engines **alongside** it? (Affects how much of the core must be
  ported vs. how much can stay behind FFI.)
- Does the reproduce **blob** format use compression (`miniz_oxide`)? If so,
  cross-language *encode* parity may be infeasible; decide whether blobs must
  round-trip across languages or only within one.
- Which target languages are actually wanted, and in what priority?
- Where should a real HegelIR live long-term if this graduates from a spike —
  its own repo/crate, or here?

## 8. Key file references

- Engine (source of truth for components):
  `hegel-c/src/native/core/float_index.rs`, `.../base64.rs`, `.../bignum.rs`,
  `.../blob.rs`, `.../rng.rs`, `hegel-c/src/native/draws/`.
- Protocol/architecture context: `docs/hegel-protocol.html` (this branch),
  `hegel-c/include/hegel.h`, `AGENTS.md`.
- Ax (design reference, **not** vendored — re-clone if needed):
  `git clone --depth 1 https://github.com/ax-llm/ax`; read `docs/COMPILER.md`,
  `ir/spec/core-ir.md`, `ir/axcore/*.axir`, and the Go compiler under
  `tools/axir/internal/axir/` (esp. `parser.go`, `check_types.go`, `*_core_emit.go`).
