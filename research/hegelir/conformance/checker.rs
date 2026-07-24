// Check the EMITTED Rust module against the Rust reference golden vectors.
// The generated functions are pulled in via include! so this compiles as one
// crate: rustc -O checker.rs (with float_index_gen.rs alongside).
#![allow(unused_parens, unused_variables, unused_mut, dead_code)]

use std::io::BufRead;

include!("float_index_gen.rs");

fn main() {
    let path = std::env::args().nth(1).expect("usage: check <vectors.tsv>");
    let file = std::fs::File::open(&path).unwrap();
    let reader = std::io::BufReader::new(file);

    let mut total: u64 = 0;
    let mut mism: u64 = 0;
    let mut counts: std::collections::BTreeMap<String, (u64, u64)> = std::collections::BTreeMap::new();
    let mut fails: Vec<String> = Vec::new();

    for line in reader.lines() {
        let line = line.unwrap();
        if line.is_empty() {
            continue;
        }
        let p: Vec<&str> = line.split('\t').collect();
        let kind = p[0];
        let (got, want, desc): (u64, u64, String) = match kind {
            "F2I" => {
                let inp = u64::from_str_radix(p[1], 16).unwrap();
                (
                    float_to_index(f64::from_bits(inp)),
                    u64::from_str_radix(p[2], 16).unwrap(),
                    format!("F2I in={}", p[1]),
                )
            }
            "I2F" => {
                let inp = u64::from_str_radix(p[1], 16).unwrap();
                (
                    index_to_float(inp).to_bits(),
                    u64::from_str_radix(p[2], 16).unwrap(),
                    format!("I2F in={}", p[1]),
                )
            }
            "SIR" => {
                let lo = u64::from_str_radix(p[1], 16).unwrap();
                let hi = u64::from_str_radix(p[2], 16).unwrap();
                (
                    simplest_in_range(f64::from_bits(lo), f64::from_bits(hi)).to_bits(),
                    u64::from_str_radix(p[3], 16).unwrap(),
                    format!("SIR lo={} hi={}", p[1], p[2]),
                )
            }
            _ => panic!("unknown kind {}", kind),
        };
        total += 1;
        let e = counts.entry(kind.to_string()).or_insert((0, 0));
        e.0 += 1;
        if got != want {
            mism += 1;
            e.1 += 1;
            if fails.len() < 10 {
                fails.push(format!("  {}  got={:016x} want={:016x}", desc, got, want));
            }
        }
    }

    println!("checked {} vectors across {} kinds", total, counts.len());
    for k in ["F2I", "I2F", "SIR"] {
        if let Some(v) = counts.get(k) {
            println!("  {:<4} {:6} checked  {:6} mismatched", k, v.0, v.1);
        }
    }
    if mism == 0 {
        println!("RESULT: PASS — emitted Rust is bit-exact with the Rust reference");
    } else {
        println!("RESULT: FAIL — {}/{} mismatched", mism, total);
        for s in &fails {
            println!("{}", s);
        }
        std::process::exit(1);
    }
}
