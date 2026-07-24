#!/usr/bin/env bash
# End-to-end HegelIR spike: Rust reference -> golden vectors, then the AxIR-style
# ir/float_index.axir -> native Go + native Python, then check both emitted
# targets are bit-exact with the reference. Requires rustc, go, and python3.
set -euo pipefail
cd "$(dirname "$0")"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "==> [1/3] Rust reference: generate golden vectors"
rustc -O reference/ref.rs -o "$TMP/ref"
"$TMP/ref" > "$TMP/vectors.tsv"
echo "    $(wc -l < "$TMP/vectors.tsv") vectors"

echo "==> [2/3] emit native Go + Python + Rust from ir/float_index.axir"
( cd emitter && go run . ../ir/float_index.axir go ) > targets/go/float_index_gen.go
( cd emitter && go run . ../ir/float_index.axir py ) > targets/python/float_index_gen.py
( cd emitter && go run . ../ir/float_index.axir rust ) > targets/rust/float_index_gen.rs
echo "    wrote targets/{go,python,rust}/float_index_gen.*"

echo "==> [3/3] conformance: each emitted target vs the Rust reference"
GOC="$TMP/goc"; mkdir -p "$GOC"
cp conformance/checker.go conformance/go.mod targets/go/float_index_gen.go "$GOC"/
( cd "$GOC" && go run . "$TMP/vectors.tsv" )
cp targets/python/float_index_gen.py conformance/py_check.py "$TMP"/
( cd "$TMP" && python3 py_check.py "$TMP/vectors.tsv" )
RSC="$TMP/rsc"; mkdir -p "$RSC"
cp conformance/checker.rs targets/rust/float_index_gen.rs "$RSC"/
( cd "$RSC" && rustc -O checker.rs -o check && ./check "$TMP/vectors.tsv" )

echo "==> validator: malformed IR must be rejected"
cat > "$TMP/bad.axir" <<'EOF'
module @bad version "0.1" {
  op core.func @f {
    body @entry(%x: u64) {
      %y = core.call intrinsic.bogus(%x, %missing)
      core.return %y
    }
  }
}
EOF
if ( cd emitter && go run . "$TMP/bad.axir" go ) >/dev/null 2>"$TMP/err"; then
  echo "    FAIL: malformed IR was accepted"; exit 1
else
  echo "    OK: rejected —"; sed 's/^/      /' "$TMP/err"
fi

echo "==> done"
