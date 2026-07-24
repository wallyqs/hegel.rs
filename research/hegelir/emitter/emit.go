// HegelIR emitter: lowers one IR module to native Go or Python.
//
// This is the "codegen leg" — the analog of Ax's Go compiler emitters. The
// same IR drives every target; each target's emitter encodes that language's
// integer/float semantics (Go: native uint64 wraparound; Python: mask to 64
// bits, struct-based float bitcast) so the output is bit-exact everywhere.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type fn struct {
	Name   string     `json:"name"`
	Params [][]string `json:"params"`
	Ret    string     `json:"ret"`
	Body   []any      `json:"body"`
}
type module struct {
	Functions []fn `json:"functions"`
}

var funcRet = map[string]string{}

func arr(x any) []any { return x.([]any) }
func s(x any) string  { return x.(string) }

func typeOf(env map[string]string, e []any) string {
	switch e[0].(string) {
	case "c":
		return s(e[1])
	case "v":
		return env[s(e[1])]
	case "call":
		return funcRet[s(e[1])]
	case "u":
		switch s(e[1]) {
		case "revbits", "f64bits", "f2u", "i2u":
			return "u64"
		case "u2i":
			return "i64"
		case "ceil", "bitsf64", "u2f":
			return "f64"
		default: // not, signbit, isnan, isinf, isfinite
			return "bool"
		}
	case "b":
		switch s(e[1]) {
		case "eq", "ne", "lt", "le", "gt", "ge", "land", "lor":
			return "bool"
		default:
			return typeOf(env, arr(e[2]))
		}
	case "sel":
		return typeOf(env, arr(e[2]))
	}
	panic("typeOf: " + s(e[0]))
}

func camel(name string) string {
	parts := strings.Split(name, "_")
	out := parts[0]
	for _, p := range parts[1:] {
		out += strings.ToUpper(p[:1]) + p[1:]
	}
	return out
}

func goType(t string) string {
	switch t {
	case "u64":
		return "uint64"
	case "i64":
		return "int64"
	case "f64":
		return "float64"
	default:
		return "bool"
	}
}

var goBin = map[string]string{"add": "+", "sub": "-", "mul": "*", "shl": "<<", "shr": ">>",
	"and": "&", "or": "|", "xor": "^", "eq": "==", "ne": "!=", "lt": "<", "le": "<=",
	"gt": ">", "ge": ">=", "land": "&&", "lor": "||"}

func goExpr(env map[string]string, e []any) string {
	switch e[0].(string) {
	case "c":
		switch s(e[1]) {
		case "u64":
			return "uint64(" + s(e[2]) + ")"
		case "i64":
			return "int64(" + s(e[2]) + ")"
		case "f64":
			return "float64(" + s(e[2]) + ")"
		default:
			return s(e[2])
		}
	case "v":
		return s(e[1])
	case "call":
		var a []string
		for _, x := range arr(e[2]) {
			a = append(a, goExpr(env, arr(x)))
		}
		return camel(s(e[1])) + "(" + strings.Join(a, ", ") + ")"
	case "u":
		x := goExpr(env, arr(e[2]))
		switch s(e[1]) {
		case "not":
			return "(!" + x + ")"
		case "revbits":
			return "bits.Reverse64(" + x + ")"
		case "signbit":
			return "math.Signbit(" + x + ")"
		case "isnan":
			return "math.IsNaN(" + x + ")"
		case "isinf":
			return "math.IsInf(" + x + ", 0)"
		case "isfinite":
			return "(!math.IsInf(" + x + ", 0) && !math.IsNaN(" + x + "))"
		case "ceil":
			return "math.Ceil(" + x + ")"
		case "f64bits":
			return "math.Float64bits(" + x + ")"
		case "bitsf64":
			return "math.Float64frombits(" + x + ")"
		case "u2f":
			return "float64(" + x + ")"
		case "f2u":
			return "uint64(" + x + ")"
		case "u2i":
			return "int64(" + x + ")"
		case "i2u":
			return "uint64(" + x + ")"
		}
	case "b":
		x := goExpr(env, arr(e[2]))
		y := goExpr(env, arr(e[3]))
		return "(" + x + " " + goBin[s(e[1])] + " " + y + ")"
	case "sel":
		cond := goExpr(env, arr(e[1]))
		a := goExpr(env, arr(e[2]))
		d := goExpr(env, arr(e[3]))
		t := goType(typeOf(env, arr(e[2])))
		return "(func() " + t + " { if " + cond + " { return " + a + " }; return " + d + " })()"
	}
	panic("goExpr")
}

var pyBin = map[string]string{"eq": "==", "ne": "!=", "lt": "<", "le": "<=", "gt": ">",
	"ge": ">=", "land": "and", "lor": "or"}

func pyExpr(env map[string]string, e []any) string {
	switch e[0].(string) {
	case "c":
		switch s(e[1]) {
		case "f64":
			return "float(" + s(e[2]) + ")"
		case "bool":
			if s(e[2]) == "true" {
				return "True"
			}
			return "False"
		default:
			return s(e[2])
		}
	case "v":
		return s(e[1])
	case "call":
		var a []string
		for _, x := range arr(e[2]) {
			a = append(a, pyExpr(env, arr(x)))
		}
		return s(e[1]) + "(" + strings.Join(a, ", ") + ")"
	case "u":
		x := pyExpr(env, arr(e[2]))
		switch s(e[1]) {
		case "not":
			return "(not " + x + ")"
		case "revbits":
			return "rev64(" + x + ")"
		case "signbit":
			return "(math.copysign(1.0, " + x + ") < 0.0)"
		case "isnan":
			return "math.isnan(" + x + ")"
		case "isinf":
			return "math.isinf(" + x + ")"
		case "isfinite":
			return "math.isfinite(" + x + ")"
		case "ceil":
			return "float(math.ceil(" + x + "))"
		case "f64bits":
			return "f2bits(" + x + ")"
		case "bitsf64":
			return "bits2f(" + x + ")"
		case "u2f":
			return "float(" + x + ")"
		case "f2u":
			return "int(" + x + ")"
		case "u2i":
			return "(" + x + ")"
		case "i2u":
			return "(" + x + " & MASK)"
		}
	case "b":
		op := s(e[1])
		x := pyExpr(env, arr(e[2]))
		y := pyExpr(env, arr(e[3]))
		u64 := typeOf(env, arr(e[2])) == "u64"
		switch op {
		case "add", "sub", "mul", "shl":
			sym := map[string]string{"add": "+", "sub": "-", "mul": "*", "shl": "<<"}[op]
			if u64 {
				return "((" + x + " " + sym + " " + y + ") & MASK)"
			}
			return "(" + x + " " + sym + " " + y + ")"
		case "shr":
			return "(" + x + " >> " + y + ")"
		case "and":
			return "(" + x + " & " + y + ")"
		case "or":
			return "(" + x + " | " + y + ")"
		case "xor":
			return "(" + x + " ^ " + y + ")"
		default:
			return "(" + x + " " + pyBin[op] + " " + y + ")"
		}
	case "sel":
		cond := pyExpr(env, arr(e[1]))
		a := pyExpr(env, arr(e[2]))
		d := pyExpr(env, arr(e[3]))
		return "(" + a + " if " + cond + " else " + d + ")"
	}
	panic("pyExpr")
}

func copyEnv(env map[string]string) map[string]string {
	n := map[string]string{}
	for k, v := range env {
		n[k] = v
	}
	return n
}

// emitBlock lowers a statement slice. Every `if` in this IR has a then-branch
// that always returns, so we flatten: emit the guarded then-block, then splice
// the else-branch in front of the remaining statements at the same level.
func emitBlock(env map[string]string, stmts []any, indent int, target string) []string {
	pad := strings.Repeat("\t", indent)
	if target == "py" {
		pad = strings.Repeat("    ", indent)
	}
	var out []string
	expr := goExpr
	if target == "py" {
		expr = pyExpr
	}
	for k := 0; k < len(stmts); k++ {
		st := arr(stmts[k])
		switch st[0].(string) {
		case "let":
			name := s(st[1])
			e := arr(st[2])
			env[name] = typeOf(env, e)
			if target == "py" {
				out = append(out, pad+name+" = "+expr(env, e))
			} else {
				out = append(out, pad+name+" := "+expr(env, e))
			}
		case "ret":
			out = append(out, pad+"return "+expr(env, arr(st[1])))
		case "if":
			cond := expr(env, arr(st[1]))
			then := arr(st[2])
			els := arr(st[3])
			if target == "py" {
				out = append(out, pad+"if "+cond+":")
				out = append(out, emitBlock(copyEnv(env), then, indent+1, target)...)
			} else {
				out = append(out, pad+"if "+cond+" {")
				out = append(out, emitBlock(copyEnv(env), then, indent+1, target)...)
				out = append(out, pad+"}")
			}
			rest := append(append([]any{}, els...), stmts[k+1:]...)
			out = append(out, emitBlock(env, rest, indent, target)...)
			return out
		}
	}
	return out
}

func emitGo(m module) string {
	var b strings.Builder
	b.WriteString("package main\n\nimport (\n\t\"math\"\n\t\"math/bits\"\n)\n")
	for _, f := range m.Functions {
		env := map[string]string{}
		var ps []string
		for _, p := range f.Params {
			env[p[0]] = p[1]
			ps = append(ps, p[0]+" "+goType(p[1]))
		}
		b.WriteString("\nfunc " + camel(f.Name) + "(" + strings.Join(ps, ", ") + ") " + goType(f.Ret) + " {\n")
		for _, line := range emitBlock(env, f.Body, 1, "go") {
			b.WriteString(line + "\n")
		}
		b.WriteString("}\n")
	}
	return b.String()
}

func emitPy(m module) string {
	var b strings.Builder
	b.WriteString("import math, struct\nMASK = (1 << 64) - 1\n")
	b.WriteString("def rev64(x):\n    r = 0\n    for _ in range(64):\n        r = ((r << 1) | (x & 1)) & MASK\n        x >>= 1\n    return r\n")
	b.WriteString("def f2bits(x): return struct.unpack('<Q', struct.pack('<d', x))[0]\n")
	b.WriteString("def bits2f(x): return struct.unpack('<d', struct.pack('<Q', x))[0]\n")
	for _, f := range m.Functions {
		env := map[string]string{}
		var ps []string
		for _, p := range f.Params {
			env[p[0]] = p[1]
			ps = append(ps, p[0])
		}
		b.WriteString("\ndef " + f.Name + "(" + strings.Join(ps, ", ") + "):\n")
		for _, line := range emitBlock(env, f.Body, 1, "py") {
			b.WriteString(line + "\n")
		}
	}
	return b.String()
}

func main() {
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	var m module
	if err := json.Unmarshal(data, &m); err != nil {
		panic(err)
	}
	for _, f := range m.Functions {
		funcRet[f.Name] = f.Ret
	}
	switch os.Args[2] {
	case "go":
		fmt.Print(emitGo(m))
	case "py":
		fmt.Print(emitPy(m))
	default:
		panic("unknown target " + os.Args[2])
	}
}
