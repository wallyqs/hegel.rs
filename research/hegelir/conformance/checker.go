package main

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

// Conformance checker: read the golden vectors produced by the Rust
// reference and verify the Go port reproduces every output bit-for-bit.

func hx(s string) uint64 {
	v, err := strconv.ParseUint(s, 16, 64)
	if err != nil {
		panic(err)
	}
	return v
}

func main() {
	f, err := os.Open(os.Args[1])
	if err != nil {
		panic(err)
	}
	defer f.Close()

	var total, mismatch int
	byKind := map[string][2]int{} // kind -> {total, mismatch}
	var firstFails []string

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		p := strings.Split(line, "\t")
		kind := p[0]
		var got, want uint64
		var desc string
		switch kind {
		case "F2I":
			in, expect := hx(p[1]), hx(p[2])
			got = floatToIndex(math.Float64frombits(in))
			want = expect
			desc = fmt.Sprintf("F2I in=%016x", in)
		case "I2F":
			in, expect := hx(p[1]), hx(p[2])
			got = math.Float64bits(indexToFloat(in))
			want = expect
			desc = fmt.Sprintf("I2F in=%016x", in)
		case "SIR":
			lo, hi, expect := hx(p[1]), hx(p[2]), hx(p[3])
			got = math.Float64bits(simplestInRange(
				math.Float64frombits(lo), math.Float64frombits(hi)))
			want = expect
			desc = fmt.Sprintf("SIR lo=%016x hi=%016x", lo, hi)
		default:
			panic("unknown kind " + kind)
		}
		total++
		agg := byKind[kind]
		agg[0]++
		if got != want {
			mismatch++
			agg[1]++
			if len(firstFails) < 10 {
				firstFails = append(firstFails, fmt.Sprintf("  %s  got=%016x want=%016x", desc, got, want))
			}
		}
		byKind[kind] = agg
	}
	if err := sc.Err(); err != nil {
		panic(err)
	}

	fmt.Printf("checked %d vectors across %d kinds\n", total, len(byKind))
	for _, k := range []string{"F2I", "I2F", "SIR"} {
		if agg, ok := byKind[k]; ok {
			fmt.Printf("  %-4s %6d checked  %6d mismatched\n", k, agg[0], agg[1])
		}
	}
	if mismatch == 0 {
		fmt.Println("RESULT: PASS — Go port is bit-exact with the Rust reference")
	} else {
		fmt.Printf("RESULT: FAIL — %d/%d mismatched\n", mismatch, total)
		for _, s := range firstFails {
			fmt.Println(s)
		}
		os.Exit(1)
	}
}
