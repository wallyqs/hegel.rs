// HegelIR emitter: parses an AxIR-style .axir module (MLIR-like text, flat SSA
// Core bodies) and lowers it to native Go or Python.
//
// The syntax is taken from Ax's AxIR (op core.func / body @entry / core.const /
// core.call @fn / core.call intrinsic.x / core.if / core.return), extended with
// the numeric vocabulary AxIR's Core lacks: the u64 type and the intrinsic.bit.*
// / intrinsic.float.* / intrinsic.cast.* families. Each target's emitter encodes
// that language's integer/float semantics so the output is bit-exact everywhere.
package main

import (
	"fmt"
	"os"
	"strings"
)

// ---------- AST ----------

type expr struct {
	kind   string // const | copy | call
	typ    string // const type
	lit    string // const literal
	src    string // copy source binding
	callee string // call target (func name or intrinsic.*)
	isIntr bool
	args   []string
}

type stmt struct {
	kind string // assign | if | return
	name string // assign target
	expr *expr
	cond string // if condition binding
	body []stmt // if body
	ret  string // return binding
	has  bool   // return has a value
}

type fn struct {
	name   string
	params [][2]string
	ret    string
	body   []stmt
}

// ---------- tokenizer ----------

func tokenize(src string) []string {
	var t []string
	ident := func(ch byte) bool {
		return ch == '_' || ch == '.' || ch >= '0' && ch <= '9' || ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z'
	}
	for i := 0; i < len(src); {
		ch := src[i]
		switch {
		case ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r':
			i++
		case ch == '{' || ch == '}' || ch == '(' || ch == ')' || ch == ',' || ch == ':' || ch == '=':
			t = append(t, string(ch))
			i++
		case ch == '"':
			j := i + 1
			for j < len(src) && src[j] != '"' {
				j++
			}
			t = append(t, src[i:j+1])
			i = j + 1
		case ch == '@' || ch == '%':
			j := i + 1
			for j < len(src) && ident(src[j]) {
				j++
			}
			t = append(t, src[i:j])
			i = j
		default:
			if ident(ch) {
				j := i
				for j < len(src) && ident(src[j]) {
					j++
				}
				t = append(t, src[i:j])
				i = j
			} else {
				i++
			}
		}
	}
	return t
}

// ---------- parser ----------

type parser struct {
	t []string
	i int
}

func (p *parser) peek() string {
	if p.i < len(p.t) {
		return p.t[p.i]
	}
	return ""
}
func (p *parser) next() string { s := p.t[p.i]; p.i++; return s }
func (p *parser) expect(s string) {
	if p.next() != s {
		panic("expected " + s + " near token " + fmt.Sprint(p.i))
	}
}
func trim(s, pre string) string { return strings.TrimPrefix(s, pre) }

func (p *parser) parseModule() []fn {
	var fns []fn
	for p.i < len(p.t) {
		if p.peek() == "op" {
			fns = append(fns, p.parseOp())
		} else {
			p.i++
		}
	}
	return fns
}

func (p *parser) parseOp() fn {
	p.expect("op")
	p.next() // opkind, e.g. core.func
	f := fn{name: trim(p.next(), "@")}
	p.expect("{")
	for p.peek() != "}" {
		if p.peek() == "body" {
			f.params, f.body = p.parseBody()
		} else {
			p.i++
		}
	}
	p.expect("}")
	return f
}

func (p *parser) parseBody() ([][2]string, []stmt) {
	p.expect("body")
	p.expect("@entry")
	p.expect("(")
	var params [][2]string
	for p.peek() != ")" {
		name := trim(p.next(), "%")
		p.expect(":")
		typ := p.next()
		params = append(params, [2]string{name, typ})
		if p.peek() == "," {
			p.next()
		}
	}
	p.expect(")")
	p.expect("{")
	body := p.parseStmts()
	p.expect("}")
	return params, body
}

func (p *parser) parseStmts() []stmt {
	var out []stmt
	for p.peek() != "}" {
		tok := p.peek()
		switch {
		case strings.HasPrefix(tok, "%"):
			name := trim(p.next(), "%")
			p.expect("=")
			out = append(out, stmt{kind: "assign", name: name, expr: p.parseExpr()})
		case tok == "core.if":
			p.next()
			cond := trim(p.next(), "%")
			p.expect("{")
			body := p.parseStmts()
			p.expect("}")
			out = append(out, stmt{kind: "if", cond: cond, body: body})
		case tok == "core.return":
			p.next()
			s := stmt{kind: "return"}
			if strings.HasPrefix(p.peek(), "%") {
				s.ret = trim(p.next(), "%")
				s.has = true
			}
			out = append(out, s)
		default:
			p.i++
		}
	}
	return out
}

func (p *parser) parseExpr() *expr {
	switch op := p.next(); op {
	case "core.const":
		return &expr{kind: "const", typ: p.next(), lit: p.next()}
	case "core.copy":
		return &expr{kind: "copy", src: trim(p.next(), "%")}
	case "core.call":
		callee := p.next()
		e := &expr{kind: "call"}
		if strings.HasPrefix(callee, "@") {
			e.callee = trim(callee, "@")
		} else {
			e.callee, e.isIntr = callee, true
		}
		p.expect("(")
		for p.peek() != ")" {
			e.args = append(e.args, trim(p.next(), "%"))
			if p.peek() == "," {
				p.next()
			}
		}
		p.expect(")")
		return e
	default:
		panic("bad expr op " + op)
	}
}

// ---------- typing ----------

var funcRet = map[string]string{}

func intrinsicType(name string, args []string, env map[string]string) string {
	switch name {
	case "intrinsic.add", "intrinsic.sub", "intrinsic.mul",
		"intrinsic.bit.and", "intrinsic.bit.or", "intrinsic.bit.xor",
		"intrinsic.bit.shl", "intrinsic.bit.shr":
		return env[args[0]]
	case "intrinsic.bit.reverse64", "intrinsic.float.to_bits",
		"intrinsic.cast.f64_to_u64", "intrinsic.cast.i64_to_u64":
		return "u64"
	case "intrinsic.cast.u64_to_i64":
		return "i64"
	case "intrinsic.float.from_bits", "intrinsic.float.ceil", "intrinsic.cast.u64_to_f64":
		return "f64"
	default: // signbit, is_nan, is_inf, eq, ne, lt, lte, gt, gte, and, or, not
		return "bool"
	}
}

func exprType(env map[string]string, e *expr) string {
	switch e.kind {
	case "const":
		return e.typ
	case "copy":
		return env[e.src]
	case "call":
		if e.isIntr {
			return intrinsicType(e.callee, e.args, env)
		}
		return funcRet[e.callee]
	}
	panic("exprType")
}

type decl struct{ name, typ string }

// inferFunc builds the type env, the func-scope declaration order (non-params,
// first-seen), and the return type.
func inferFunc(f fn) (map[string]string, []decl, string) {
	env := map[string]string{}
	isParam := map[string]bool{}
	for _, p := range f.params {
		env[p[0]] = p[1]
		isParam[p[0]] = true
	}
	var decls []decl
	ret := ""
	var walk func(ss []stmt)
	walk = func(ss []stmt) {
		for _, s := range ss {
			switch s.kind {
			case "assign":
				t := exprType(env, s.expr)
				if _, seen := env[s.name]; !seen {
					env[s.name] = t
					if !isParam[s.name] {
						decls = append(decls, decl{s.name, t})
					}
				}
			case "if":
				walk(s.body)
			case "return":
				if s.has && ret == "" {
					ret = s.ret
				}
			}
		}
	}
	walk(f.body)
	return env, decls, env[ret]
}

// ---------- helpers ----------

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

// ---------- Go emitter ----------

func riGo(name string, a []string) string {
	bin := func(sym string) string { return "(" + a[0] + " " + sym + " " + a[1] + ")" }
	switch name {
	case "intrinsic.add":
		return bin("+")
	case "intrinsic.sub":
		return bin("-")
	case "intrinsic.mul":
		return bin("*")
	case "intrinsic.bit.and":
		return bin("&")
	case "intrinsic.bit.or":
		return bin("|")
	case "intrinsic.bit.xor":
		return bin("^")
	case "intrinsic.bit.shl":
		return bin("<<")
	case "intrinsic.bit.shr":
		return bin(">>")
	case "intrinsic.bit.reverse64":
		return "bits.Reverse64(" + a[0] + ")"
	case "intrinsic.float.to_bits":
		return "math.Float64bits(" + a[0] + ")"
	case "intrinsic.float.from_bits":
		return "math.Float64frombits(" + a[0] + ")"
	case "intrinsic.float.ceil":
		return "math.Ceil(" + a[0] + ")"
	case "intrinsic.float.signbit":
		return "math.Signbit(" + a[0] + ")"
	case "intrinsic.float.is_nan":
		return "math.IsNaN(" + a[0] + ")"
	case "intrinsic.float.is_inf":
		return "math.IsInf(" + a[0] + ", 0)"
	case "intrinsic.cast.u64_to_f64":
		return "float64(" + a[0] + ")"
	case "intrinsic.cast.f64_to_u64":
		return "uint64(" + a[0] + ")"
	case "intrinsic.cast.u64_to_i64":
		return "int64(" + a[0] + ")"
	case "intrinsic.cast.i64_to_u64":
		return "uint64(" + a[0] + ")"
	case "intrinsic.eq":
		return bin("==")
	case "intrinsic.ne":
		return bin("!=")
	case "intrinsic.lt":
		return bin("<")
	case "intrinsic.lte":
		return bin("<=")
	case "intrinsic.gt":
		return bin(">")
	case "intrinsic.gte":
		return bin(">=")
	case "intrinsic.and":
		return bin("&&")
	case "intrinsic.or":
		return bin("||")
	case "intrinsic.not":
		return "(!" + a[0] + ")"
	}
	panic("riGo " + name)
}

func exprGo(e *expr) string {
	switch e.kind {
	case "const":
		switch e.typ {
		case "u64":
			return "uint64(" + e.lit + ")"
		case "i64":
			return "int64(" + e.lit + ")"
		case "f64":
			return "float64(" + e.lit + ")"
		default:
			return e.lit
		}
	case "copy":
		return e.src
	case "call":
		if e.isIntr {
			return riGo(e.callee, e.args)
		}
		return camel(e.callee) + "(" + strings.Join(e.args, ", ") + ")"
	}
	panic("exprGo")
}

func goStmts(ss []stmt, indent int, b *strings.Builder) {
	pad := strings.Repeat("\t", indent)
	for _, s := range ss {
		switch s.kind {
		case "assign":
			b.WriteString(pad + s.name + " = " + exprGo(s.expr) + "\n")
		case "if":
			b.WriteString(pad + "if " + s.cond + " {\n")
			goStmts(s.body, indent+1, b)
			b.WriteString(pad + "}\n")
		case "return":
			if s.has {
				b.WriteString(pad + "return " + s.ret + "\n")
			} else {
				b.WriteString(pad + "return\n")
			}
		}
	}
}

func emitGo(fns []fn) string {
	var b strings.Builder
	b.WriteString("package main\n\nimport (\n\t\"math\"\n\t\"math/bits\"\n)\n")
	for _, f := range fns {
		_, decls, ret := inferFunc(f)
		var ps []string
		for _, p := range f.params {
			ps = append(ps, p[0]+" "+goType(p[1]))
		}
		b.WriteString("\nfunc " + camel(f.name) + "(" + strings.Join(ps, ", ") + ") " + goType(ret) + " {\n")
		for _, d := range decls {
			b.WriteString("\tvar " + d.name + " " + goType(d.typ) + "\n")
		}
		goStmts(f.body, 1, &b)
		b.WriteString("}\n")
	}
	return b.String()
}

// ---------- Python emitter ----------

func riPy(name string, a []string, env map[string]string) string {
	bin := func(sym string) string { return "(" + a[0] + " " + sym + " " + a[1] + ")" }
	mask := func(sym string) string {
		if env[a[0]] == "u64" {
			return "((" + a[0] + " " + sym + " " + a[1] + ") & MASK)"
		}
		return bin(sym)
	}
	switch name {
	case "intrinsic.add":
		return mask("+")
	case "intrinsic.sub":
		return mask("-")
	case "intrinsic.mul":
		return mask("*")
	case "intrinsic.bit.shl":
		return mask("<<")
	case "intrinsic.bit.shr":
		return bin(">>")
	case "intrinsic.bit.and":
		return bin("&")
	case "intrinsic.bit.or":
		return bin("|")
	case "intrinsic.bit.xor":
		return bin("^")
	case "intrinsic.bit.reverse64":
		return "rev64(" + a[0] + ")"
	case "intrinsic.float.to_bits":
		return "f2bits(" + a[0] + ")"
	case "intrinsic.float.from_bits":
		return "bits2f(" + a[0] + ")"
	case "intrinsic.float.ceil":
		return "float(math.ceil(" + a[0] + "))"
	case "intrinsic.float.signbit":
		return "(math.copysign(1.0, " + a[0] + ") < 0.0)"
	case "intrinsic.float.is_nan":
		return "math.isnan(" + a[0] + ")"
	case "intrinsic.float.is_inf":
		return "math.isinf(" + a[0] + ")"
	case "intrinsic.cast.u64_to_f64":
		return "float(" + a[0] + ")"
	case "intrinsic.cast.f64_to_u64":
		return "int(" + a[0] + ")"
	case "intrinsic.cast.u64_to_i64":
		return "(" + a[0] + ")"
	case "intrinsic.cast.i64_to_u64":
		return "(" + a[0] + " & MASK)"
	case "intrinsic.eq":
		return bin("==")
	case "intrinsic.ne":
		return bin("!=")
	case "intrinsic.lt":
		return bin("<")
	case "intrinsic.lte":
		return bin("<=")
	case "intrinsic.gt":
		return bin(">")
	case "intrinsic.gte":
		return bin(">=")
	case "intrinsic.and":
		return "(" + a[0] + " and " + a[1] + ")"
	case "intrinsic.or":
		return "(" + a[0] + " or " + a[1] + ")"
	case "intrinsic.not":
		return "(not " + a[0] + ")"
	}
	panic("riPy " + name)
}

func exprPy(e *expr, env map[string]string) string {
	switch e.kind {
	case "const":
		switch e.typ {
		case "f64":
			return "float(" + e.lit + ")"
		case "bool":
			if e.lit == "true" {
				return "True"
			}
			return "False"
		default:
			return e.lit
		}
	case "copy":
		return e.src
	case "call":
		if e.isIntr {
			return riPy(e.callee, e.args, env)
		}
		return e.callee + "(" + strings.Join(e.args, ", ") + ")"
	}
	panic("exprPy")
}

func pyStmts(ss []stmt, indent int, env map[string]string, b *strings.Builder) {
	pad := strings.Repeat("    ", indent)
	for _, s := range ss {
		switch s.kind {
		case "assign":
			b.WriteString(pad + s.name + " = " + exprPy(s.expr, env) + "\n")
		case "if":
			b.WriteString(pad + "if " + s.cond + ":\n")
			pyStmts(s.body, indent+1, env, b)
		case "return":
			if s.has {
				b.WriteString(pad + "return " + s.ret + "\n")
			} else {
				b.WriteString(pad + "return\n")
			}
		}
	}
}

func emitPy(fns []fn) string {
	var b strings.Builder
	b.WriteString("import math, struct\nMASK = (1 << 64) - 1\n")
	b.WriteString("def rev64(x):\n    r = 0\n    for _ in range(64):\n        r = ((r << 1) | (x & 1)) & MASK\n        x >>= 1\n    return r\n")
	b.WriteString("def f2bits(x): return struct.unpack('<Q', struct.pack('<d', x))[0]\n")
	b.WriteString("def bits2f(x): return struct.unpack('<d', struct.pack('<Q', x))[0]\n")
	for _, f := range fns {
		env, _, _ := inferFunc(f)
		var ps []string
		for _, p := range f.params {
			ps = append(ps, p[0])
		}
		b.WriteString("\ndef " + f.name + "(" + strings.Join(ps, ", ") + "):\n")
		pyStmts(f.body, 1, env, &b)
	}
	return b.String()
}

// ---------- Rust emitter ----------

func rustType(t string) string {
	switch t {
	case "u64":
		return "u64"
	case "i64":
		return "i64"
	case "f64":
		return "f64"
	default:
		return "bool"
	}
}

func riRust(name string, a []string) string {
	bin := func(sym string) string { return "(" + a[0] + " " + sym + " " + a[1] + ")" }
	switch name {
	case "intrinsic.add":
		return bin("+")
	case "intrinsic.sub":
		return bin("-")
	case "intrinsic.mul":
		return bin("*")
	case "intrinsic.bit.and":
		return bin("&")
	case "intrinsic.bit.or":
		return bin("|")
	case "intrinsic.bit.xor":
		return bin("^")
	case "intrinsic.bit.shl":
		return bin("<<")
	case "intrinsic.bit.shr":
		return bin(">>")
	case "intrinsic.bit.reverse64":
		return a[0] + ".reverse_bits()"
	case "intrinsic.float.to_bits":
		return a[0] + ".to_bits()"
	case "intrinsic.float.from_bits":
		return "f64::from_bits(" + a[0] + ")"
	case "intrinsic.float.ceil":
		return a[0] + ".ceil()"
	case "intrinsic.float.signbit":
		return a[0] + ".is_sign_negative()"
	case "intrinsic.float.is_nan":
		return a[0] + ".is_nan()"
	case "intrinsic.float.is_inf":
		return a[0] + ".is_infinite()"
	case "intrinsic.cast.u64_to_f64":
		return "(" + a[0] + " as f64)"
	case "intrinsic.cast.f64_to_u64":
		return "(" + a[0] + " as u64)"
	case "intrinsic.cast.u64_to_i64":
		return "(" + a[0] + " as i64)"
	case "intrinsic.cast.i64_to_u64":
		return "(" + a[0] + " as u64)"
	case "intrinsic.eq":
		return bin("==")
	case "intrinsic.ne":
		return bin("!=")
	case "intrinsic.lt":
		return bin("<")
	case "intrinsic.lte":
		return bin("<=")
	case "intrinsic.gt":
		return bin(">")
	case "intrinsic.gte":
		return bin(">=")
	case "intrinsic.and":
		return bin("&&")
	case "intrinsic.or":
		return bin("||")
	case "intrinsic.not":
		return "(!" + a[0] + ")"
	}
	panic("riRust " + name)
}

func exprRust(e *expr) string {
	switch e.kind {
	case "const":
		switch e.typ {
		case "u64":
			return e.lit + "u64"
		case "i64":
			return e.lit + "i64"
		case "f64":
			return e.lit + "f64"
		default:
			return e.lit
		}
	case "copy":
		return e.src
	case "call":
		if e.isIntr {
			return riRust(e.callee, e.args)
		}
		return e.callee + "(" + strings.Join(e.args, ", ") + ")"
	}
	panic("exprRust")
}

func rustStmts(ss []stmt, indent int, b *strings.Builder) {
	pad := strings.Repeat("    ", indent)
	for _, s := range ss {
		switch s.kind {
		case "assign":
			b.WriteString(pad + s.name + " = " + exprRust(s.expr) + ";\n")
		case "if":
			b.WriteString(pad + "if " + s.cond + " {\n")
			rustStmts(s.body, indent+1, b)
			b.WriteString(pad + "}\n")
		case "return":
			if s.has {
				b.WriteString(pad + "return " + s.ret + ";\n")
			} else {
				b.WriteString(pad + "return;\n")
			}
		}
	}
}

func emitRust(fns []fn) string {
	var b strings.Builder
	for _, f := range fns {
		_, decls, ret := inferFunc(f)
		var ps []string
		for _, p := range f.params {
			ps = append(ps, p[0]+": "+rustType(p[1]))
		}
		b.WriteString("\nfn " + f.name + "(" + strings.Join(ps, ", ") + ") -> " + rustType(ret) + " {\n")
		for _, d := range decls {
			b.WriteString("    let mut " + d.name + ": " + rustType(d.typ) + ";\n")
		}
		rustStmts(f.body, 1, &b)
		b.WriteString("}\n")
	}
	return b.String()
}

func main() {
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	p := &parser{t: tokenize(string(data))}
	fns := p.parseModule()
	for _, f := range fns {
		_, _, ret := inferFunc(f)
		funcRet[f.name] = ret
	}
	switch os.Args[2] {
	case "go":
		fmt.Print(emitGo(fns))
	case "py":
		fmt.Print(emitPy(fns))
	case "rust":
		fmt.Print(emitRust(fns))
	default:
		panic("unknown target " + os.Args[2])
	}
}
