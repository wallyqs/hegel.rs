#!/usr/bin/env bash
# End-to-end HegelIR spike: Rust reference -> golden vectors, IR -> native Go +
# native Python, then check both emitted targets are bit-exact with the
# reference. Requires rustc, go, and python3 on PATH.
set -euo pipefail
cd "$(dirname "$0")"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "==> [1/4] Rust reference: generate golden vectors"
rustc -O reference/ref.rs -o "$TMP/ref"
"$TMP/ref" > "$TMP/vectors.tsv"
echo "    $(wc -l < "$TMP/vectors.tsv") vectors"

echo "==> [2/4] author the IR (regenerate ir/float_index.ir.json)"
( cd ir && python3 build_ir.py )

echo "==> [3/4] emit native Go + Python from the ONE IR"
( cd emitter && go run . ../ir/float_index.ir.json go ) > targets/go/float_index_gen.go
( cd emitter && go run . ../ir/float_index.ir.json py ) > targets/python/float_index_gen.py
echo "    wrote targets/go/float_index_gen.go + targets/python/float_index_gen.py"

echo "==> [4/4] conformance: each emitted target vs the Rust reference"
GOC="$TMP/goc"; mkdir -p "$GOC"
cp conformance/checker.go conformance/go.mod targets/go/float_index_gen.go "$GOC"/
( cd "$GOC" && go run . "$TMP/vectors.tsv" )
cp targets/python/float_index_gen.py conformance/py_check.py "$TMP"/
( cd "$TMP" && python3 py_check.py "$TMP/vectors.tsv" )

echo "==> done"
