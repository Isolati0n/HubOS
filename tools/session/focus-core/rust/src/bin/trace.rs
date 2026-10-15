//! Runs a shared trace file (tools/session/focus-core/traces/*.trace) through the Rust core and compares every step.
//! Usage: trace FILE...   Exit code 0 if every step of every scenario matches, 1 otherwise.
use focus_core::*;
use std::fmt::Write as _;

fn act_line(a: &Act) -> String {
    match a.t {
        ActKind::Deliver => format!("D {} {} {}", a.dest, Kind::NAMES[a.kind as usize], a.code),
        ActKind::DeliverHub => format!("H {} {}", Kind::NAMES[a.kind as usize], a.code),
        ActKind::Drop => "X".to_string(),
        ActKind::Flash => format!("F {}", a.dest),
        ActKind::SynthKu => format!("SK {} {}", a.dest, a.code),
        ActKind::SynthBu => format!("SB {} {}", a.dest, a.code),
        ActKind::ReleaseAll => format!("RA {}", a.dest),
    }
}

fn list(vals: impl Iterator<Item = String>) -> String {
    let v: Vec<String> = vals.collect();
    if v.is_empty() { "-".to_string() } else { v.join(",") }
}

fn held_list(h: &[Held], n: usize) -> String {
    let mut v: Vec<(u8, u32)> = h[..n].iter().map(|x| (x.dest, x.code)).collect();
    v.sort();
    list(v.iter().map(|(d, c)| format!("{}:{}", d, c)))
}

fn state_line(s: &State, n: usize) -> String {
    let mut o = String::new();
    let w = if s.waiting == NONE { "-".to_string() } else { s.waiting.to_string() };
    write!(o, "state focus={} epoch={} waiting={} ", s.focus, s.epoch, w).unwrap();
    write!(o, "gen={} ", list((1..=n).map(|m| s.gen[m].to_string()))).unwrap();
    write!(o, "conn={} ", list((1..=n).map(|m| (s.conn[m] as u8).to_string()))).unwrap();
    write!(o, "owed={} ", list((1..=n).map(|m| (s.owed[m] as u8).to_string()))).unwrap();
    write!(o, "keys={} btns={}", held_list(&s.keys, s.nkeys), held_list(&s.btns, s.nbtns)).unwrap();
    o
}

fn parse_event(line: &str) -> Option<Event> {
    let f: Vec<&str> = line.split_whitespace().collect();
    if f.len() != 6 || f[0] != "ev" {
        return None;
    }
    let kind = Kind::NAMES.iter().position(|n| *n == f[1]).map(|i| Kind::ALL[i])?;
    Some(Event { kind, a: f[2].parse().ok()?, code: f[3].parse().ok()?, gen: f[4].parse().ok()?, ep: f[5].parse().ok()? })
}

fn main() {
    let mut steps = 0usize;
    let mut scenarios = 0usize;
    let mut bad = 0usize;
    for path in std::env::args().skip(1) {
        let text = match std::fs::read_to_string(&path) {
            Ok(t) => t,
            Err(e) => {
                eprintln!("{}: {}", path, e);
                std::process::exit(1);
            }
        };
        let mut n = 3usize;
        let mut st = State::new();
        let mut out = Out::new();
        let mut name = String::new();
        let mut pending: Option<Event> = None;
        let mut want_acts: Vec<String> = Vec::new();
        for (ln, line) in text.lines().enumerate() {
            let line = line.trim();
            if line.is_empty() || line.starts_with('#') {
                continue;
            }
            if let Some(v) = line.strip_prefix("n ") {
                n = v.parse().unwrap();
            } else if let Some(v) = line.strip_prefix("scenario ") {
                name = v.to_string();
                st = State::new();
                scenarios += 1;
            } else if line == "end" {
                // nothing
            } else if line.starts_with("ev ") {
                pending = parse_event(line);
                if pending.is_none() {
                    eprintln!("{}:{}: bad event line", path, ln + 1);
                    bad += 1;
                }
                want_acts.clear();
            } else if line.starts_with("act ") {
                want_acts.push(line[4..].to_string());
            } else if line.starts_with("state ") {
                if let Some(ev) = pending.take() {
                    st.step(&ev, &mut out);
                    steps += 1;
                    let mut got: Vec<String> = out.acts[..out.n].iter().map(act_line).collect();
                    got.sort();
                    let mut want = want_acts.clone();
                    want.sort();
                    if got != want {
                        eprintln!("{}:{} [{}] actions differ\n  want {:?}\n  got  {:?}", path, ln + 1, name, want, got);
                        bad += 1;
                    }
                    let gs = state_line(&st, n);
                    if gs != line {
                        eprintln!("{}:{} [{}] state differs\n  want {}\n  got  {}", path, ln + 1, name, line, gs);
                        bad += 1;
                    }
                }
            }
        }
    }
    println!("rust trace: scenarios={} steps={} mismatches={}", scenarios, steps, bad);
    if bad > 0 {
        std::process::exit(1);
    }
}
