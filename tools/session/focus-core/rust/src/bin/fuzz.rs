//! Fuzz run for the Rust focus core: random event sequences, the invariants checked after EVERY step by the
//! independent checker (focus_core::check). A case is one sequence of 8..32 events from a fresh state.
//! Usage: fuzz SEED CASES     Prints one summary line; exit code 1 on the first violated invariant.
//! Overflow checks and debug assertions are ON (Cargo.toml): a panic is a failure. The same generator and the same
//! hash are implemented in the Zig and Ada candidates; equal hashes for equal (SEED, CASES) mean identical behaviour.
use focus_core::check::{after_step, Ghost};
use focus_core::*;
use std::time::Instant;

struct Rng(u64);
impl Rng {
    fn next(&mut self) -> u64 {
        // splitmix64
        self.0 = self.0.wrapping_add(0x9E37_79B9_7F4A_7C15);
        let mut z = self.0;
        z = (z ^ (z >> 30)).wrapping_mul(0xBF58_476D_1CE4_E5B9);
        z = (z ^ (z >> 27)).wrapping_mul(0x94D0_49BB_1331_11EB);
        z ^ (z >> 31)
    }
    fn choose(&mut self, n: u32) -> u32 {
        ((self.next() >> 32) as u32) % n
    }
}

struct Hash(u64);
impl Hash {
    fn feed(&mut self, v: u32) {
        for b in v.to_le_bytes() {
            self.0 = (self.0 ^ b as u64).wrapping_mul(0x0000_0100_0000_01B3);
        }
    }
}

fn gen_event(r: &mut Rng, st: &State, nm: u32) -> Event {
    let k = r.choose(100);
    let stamp = |r: &mut Rng, d: u8| -> (u32, u32) {
        let g = if (d as usize) <= MAX_M && r.choose(10) < 8 { st.gen[d as usize] } else { r.choose(3) };
        let e = if r.choose(10) < 8 { st.epoch } else { r.choose(st.epoch.saturating_add(2)) };
        (g, e)
    };
    let dest = |r: &mut Rng| -> u8 { if r.choose(4) < 3 { st.focus } else { r.choose(nm + 1) as u8 } };
    if k < 50 {
        let sub = r.choose(5);
        let (kind, code) = match sub {
            0 | 1 => {
                let c = if r.choose(16) == 0 { r.choose(64) } else { r.choose(8) };
                (if sub == 0 { Kind::Kd } else { Kind::Ku }, c)
            }
            2 | 3 => (if sub == 2 { Kind::Bd } else { Kind::Bu }, r.choose(4)),
            _ => (Kind::Motion, 0),
        };
        let d = dest(r);
        let (g, e) = stamp(r, d);
        return Event { kind, a: d, code, gen: g, ep: e };
    }
    if k < 55 {
        let kind = if r.choose(2) == 0 { Kind::Chord } else { Kind::Menu };
        return Event { kind, a: HUB, code: 0, gen: 0, ep: 0 };
    }
    if k < 70 {
        let t = if r.choose(32) == 0 { 200 } else { r.choose(nm + 1) as u8 };
        return Event { kind: Kind::Req, a: t, code: 0, gen: 0, ep: 0 };
    }
    if k < 80 {
        let m = if st.waiting != NONE && r.choose(10) < 7 { st.waiting } else { (r.choose(nm) + 1) as u8 };
        let e = if r.choose(10) < 8 { st.epoch } else { r.choose(st.epoch.saturating_add(2)) };
        let kind = if r.choose(2) == 0 { Kind::Vok } else { Kind::Vto };
        return Event { kind, a: m, code: 0, gen: 0, ep: e };
    }
    let m = (r.choose(nm) + 1) as u8;
    let kind = if k < 86 { Kind::Rst } else if k < 93 { Kind::Dis } else { Kind::Rec };
    Event { kind, a: m, code: 0, gen: 0, ep: 0 }
}

fn main() {
    let args: Vec<String> = std::env::args().collect();
    let seed: u64 = args.get(1).and_then(|s| s.parse().ok()).unwrap_or(1);
    let cases: u64 = args.get(2).and_then(|s| s.parse().ok()).unwrap_or(1_000_000);
    let mut rng = Rng(seed);
    let mut h = Hash(0xcbf2_9ce4_8422_2325);
    let mut steps: u64 = 0;
    let t0 = Instant::now();
    let mut out = Out::new();
    for case in 0..cases {
        let nm = 3 + rng.choose(18);
        let len = 8 + rng.choose(25);
        let mut st = State::new();
        let mut ghost = Ghost::new();
        for step in 0..len {
            let ev = gen_event(&mut rng, &st, nm);
            let pre = st;
            st.step(&ev, &mut out);
            steps += 1;
            let bad = after_step(&pre, &ev, &out, &st, &mut ghost);
            if bad != 0 {
                eprintln!("VIOLATION invariant {} at case {} step {} event {:?} (seed {})", bad, case, step, ev, seed);
                std::process::exit(1);
            }
            h.feed(out.n as u32);
            for a in &out.acts[..out.n] {
                h.feed(a.t as u32);
                h.feed(a.dest as u32);
                h.feed(a.kind as u32);
                h.feed(a.code);
                h.feed(a.gen);
                h.feed(a.ep);
            }
            h.feed(st.focus as u32);
            h.feed(st.epoch);
            h.feed(st.waiting as u32);
            h.feed(st.nkeys as u32);
            h.feed(st.nbtns as u32);
            let mut gsum: u32 = 0;
            let mut cbits: u32 = 0;
            for m in 1..=MAX_M {
                gsum = gsum.wrapping_add(st.gen[m]);
                cbits += st.conn[m] as u32;
            }
            h.feed(gsum);
            h.feed(cbits);
        }
    }
    let secs = t0.elapsed().as_secs_f64();
    println!(
        "rust fuzz: seed={} cases={} steps={} violations=0 hash={:016x} seconds={:.2} steps_per_second={:.0}",
        seed, cases, steps, h.0, secs, steps as f64 / secs.max(1e-9)
    );
}
