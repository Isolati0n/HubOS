//! hsfuzz: in-process mutation fuzzer for driftwm's pure-logic code (bulletproof-compositor research).
//! Targets: toml config, action / key / mouse / gesture parsers, session.json (serde + read from disk),
//! IPC Request JSON, canvas math, region decomposition, fill_rect, place_auto, cluster shifts.
//! Every call runs under catch_unwind; a panic is counted per target and its input is saved (first 5 per message).
//! Build in debug for overflow checks (the UBSan-equivalent Rust has) and in release.
//! usage: hsfuzz TARGET SECONDS SEED OUTDIR
use std::collections::{HashMap, HashSet};
use std::panic::{catch_unwind, AssertUnwindSafe};
use std::time::{Duration, Instant};

#[path = "../src/ipc/protocol.rs"]
#[allow(dead_code)]
mod protocol;

#[path = "../src/region.rs"]
#[allow(dead_code)]
mod region;

use smithay::utils::{Logical, Point, Rectangle, Size};

struct Rng(u64);
impl Rng {
    fn next(&mut self) -> u64 { self.0 ^= self.0 << 13; self.0 ^= self.0 >> 7; self.0 ^= self.0 << 17; self.0 }
    fn below(&mut self, n: usize) -> usize { if n == 0 { 0 } else { (self.next() % n as u64) as usize } }
    fn f(&mut self) -> f64 {
        const S: [f64; 14] = [0.0, -0.0, 1.0, -1.0, 0.5, 1e-9, 1e9, 1e300, -1e300, f64::NAN, f64::INFINITY, f64::NEG_INFINITY, f64::MIN_POSITIVE, 4294967296.0];
        if self.below(3) == 0 { S[self.below(S.len())] } else { (self.next() % 20001) as f64 / 10.0 - 1000.0 }
    }
    fn i(&mut self) -> i32 {
        const S: [i32; 10] = [0, 1, -1, i32::MAX, i32::MIN, i32::MAX - 1, i32::MIN + 1, 65536, -65536, 1 << 30];
        if self.below(3) == 0 { S[self.below(S.len())] } else { (self.next() % 4001) as i32 - 2000 }
    }
}

fn mutate(rng: &mut Rng, seed: &[u8]) -> Vec<u8> {
    let mut v = seed.to_vec();
    for _ in 0..1 + rng.below(8) {
        if v.is_empty() { v.push(b'a'); }
        let p = rng.below(v.len());
        match rng.below(8) {
            0 => v[p] ^= 1 << rng.below(8),
            1 => v[p] = rng.next() as u8,
            2 => { v.remove(p); }
            3 => { let c = v[p]; v.insert(p, c); }
            4 => { let ins: &[&[u8]] = &["é".as_bytes(), "漢".as_bytes(), b"\"", b"\\", b"[", b"]", b"=", b"\n", b"#", b"\0", b"-1", b"99999999999999999999", b"nan", b"\xff", "\u{1F600}".as_bytes(), b"{", b"}"]; let s = ins[rng.below(ins.len())]; for (k, b) in s.iter().enumerate() { v.insert(p + k, *b); } }
            5 => { let q = rng.below(v.len()); let (a, b) = if p < q { (p, q) } else { (q, p) }; let chunk = v[a..b].to_vec(); let at = rng.below(v.len()); for (k, c) in chunk.iter().enumerate() { v.insert(at + k, *c); } }
            6 => { let n = rng.below(v.len()); v.truncate(n.max(1)); }
            _ => { v.extend_from_slice(b"\n"); }
        }
        if v.len() > 200_000 { v.truncate(200_000); }
    }
    v
}

fn sf(x: f64) -> f64 { if x.is_nan() { 0.0 } else { x.abs() } }

fn main() {
    let a: Vec<String> = std::env::args().collect();
    let (target, secs, seed, out) = (a[1].clone(), a[2].parse::<u64>().unwrap(), a[3].parse::<u64>().unwrap(), a[4].clone());
    std::fs::create_dir_all(&out).unwrap();
    let mut rng = Rng(seed.wrapping_mul(0x9E3779B97F4A7C15) | 1);
    std::panic::set_hook(Box::new(|i| { if std::env::var_os("HSF_SHOWPANIC").is_some() { eprintln!("{i}"); } }));
    let end = Instant::now() + Duration::from_secs(secs);
    let mut iters: u64 = 0;
    let mut panics: HashMap<String, (u64, usize)> = HashMap::new();
    let seeds_toml: Vec<Vec<u8>> = vec![include_bytes!("../config.reference.toml").to_vec(), b"[session]\nrestore_windows = true\n[decorations]\nbg_color = \"#112233\"\n".to_vec()];
    let seeds_action: Vec<&str> = vec!["spawn foot", "close-window", "switch-layout next", "move-window left 20", "pan-viewport up", "zoom-in", "exec sh -c 'x'", "goto-bookmark a", "set-bookmark a", "fit-window", "fill-window", "nudge-window right", "send-to-output next", "center-nearest left", "suspend-window", "toggle-fullscreen", "scroll-up", "resize-window 10 10"];
    let seeds_combo: Vec<&str> = vec!["mod+shift+a", "alt+left", "super+return", "mod+ctrl+shift+Tab", "ctrl+alt+Delete", "mod+scroll", "mod+left-click", "3finger-swipe-left", "mod+3finger-pinch-in", "touch-3finger-swipe-up", "mod+XF86AudioMute"];
    let seeds_json: Vec<&str> = vec![r#"{"version":2,"saved_at":0,"entries":[{"id":1,"app_id":"a","desktop_id":"a.desktop","display_name":"A","position":[1,2],"size":[3,4],"origin":"explicit"}],"outputs":{"x":{"camera":[0.0,0.0],"zoom":1.0}},"bookmarks":{"b":[1.0,2.0]}}"#,
        r#"{"Move":{"window":5,"to":[1,2]}}"#, r#"{"Resize":{"window":"term","to":[10,20]}}"#, r#"{"Camera":[1.5,2.5]}"#, r#""State""#, r#"{"Action":"close-window"}"#, r#"{"Bookmark":{"name":"a","to":[1.0,2.0],"delete":false}}"#, r#"{"Screenshot":{"target":{"Region":{"x":1,"y":2,"w":3,"h":4,"from_screen":false}},"scale":1.0,"path":"/tmp/x.png"}}"#, r#"{"Opacity":{"window":null,"value":0.5}}"#];
    let tmpdir = format!("{out}/tmp");
    std::fs::create_dir_all(&tmpdir).unwrap();
    macro_rules! guard { ($name:expr, $input:expr, $body:expr) => {{
        let r = catch_unwind(AssertUnwindSafe(|| { $body }));
        if let Err(e) = r {
            let msg = if let Some(s) = e.downcast_ref::<String>() { s.clone() } else if let Some(s) = e.downcast_ref::<&str>() { s.to_string() } else { "?".into() };
            let key = format!("{}: {}", $name, msg.chars().take(58).collect::<String>());
            let ent = panics.entry(key.clone()).or_insert((0, 0)); ent.0 += 1;
            if ent.1 < 3 { ent.1 += 1; let fname = format!("{}/panic-{}-{}-{}.input", out, $name, seed, iters); let _ = std::fs::write(&fname, format!("{:?}", $input)); }
        }
    }}; }
    while Instant::now() < end {
        for _ in 0..200 {
            iters += 1;
            match target.as_str() {
                "toml" => {
                    let pick = seeds_toml[rng.below(seeds_toml.len())].clone();
                    let s = mutate(&mut rng, &pick);
                    let s = String::from_utf8_lossy(&s).to_string();
                    guard!("toml", &s, { let _ = driftwm::config::Config::from_toml_collect(&s); });
                }
                "parse" => {
                    let pick = if rng.below(2) == 0 { seeds_action[rng.below(seeds_action.len())].as_bytes() } else { seeds_combo[rng.below(seeds_combo.len())].as_bytes() };
                    let s = mutate(&mut rng, pick);
                    let s = String::from_utf8_lossy(&s).to_string();
                    guard!("parse_action", &s, { let _ = driftwm::config::parse_action(&s); });
                    guard!("parse_key_combo", &s, { let _ = driftwm::config::parse_key_combo(&s, driftwm::config::ModKey::Alt); });
                    guard!("parse_mouse_binding", &s, { let _ = driftwm::config::parse_mouse_binding(&s, driftwm::config::ModKey::Alt); });
                    guard!("parse_gesture_binding", &s, { let _ = driftwm::config::parse_gesture_binding(&s, driftwm::config::ModKey::Super); });
                    guard!("parse_gesture_trigger", &s, { let _ = driftwm::config::parse_gesture_trigger(&s); });
                    guard!("parse_touch_trigger", &s, { let _ = driftwm::config::parse_touch_trigger(&s); });
                    guard!("parse_tap_combo", &s, { let _ = driftwm::config::parse_tap_combo(&s, driftwm::config::ModKey::Alt); });
                }
                "json" => {
                    let pick = seeds_json[rng.below(seeds_json.len())].as_bytes();
                    let s = mutate(&mut rng, pick);
                    let st = String::from_utf8_lossy(&s).to_string();
                    guard!("ipc_request", &st, { let _ = serde_json::from_str::<protocol::Request>(&st); });
                    guard!("session_envelope", &st, { let _ = serde_json::from_str::<driftwm::session::SessionEnvelope>(&st); });
                    if iters % 50 == 0 {
                        let p = std::path::PathBuf::from(format!("{tmpdir}/session-{}.json", seed));
                        let _ = std::fs::write(&p, &s);
                        guard!("session_read_disk", &st, { let _ = driftwm::session::read(&p); });
                        // quarantine leftovers
                        if let Ok(rd) = std::fs::read_dir(&tmpdir) { for e in rd.flatten() { if e.file_name().to_string_lossy().contains("corrupt") || e.file_name().to_string_lossy().contains("unreadable") { let _ = std::fs::remove_file(e.path()); } } }
                    }
                }
                "geom" => {
                    let (cx, cy) = (rng.f(), rng.f());
                    let cam: Point<f64, Logical> = Point::from((rng.f(), rng.f()));
                    let vp: Size<i32, Logical> = Size::from((rng.i().saturating_abs(), rng.i().saturating_abs()));
                    let z = rng.f();
                    let loc: Point<i32, Logical> = Point::from((rng.i(), rng.i()));
                    let sz: Size<i32, Logical> = Size::from((rng.i().saturating_abs(), rng.i().saturating_abs()));
                    let inp = (cx, cy, cam, vp, z, loc, sz);
                    guard!("canvas_viewport_center", &inp, { let _ = driftwm::canvas::viewport_center(cam, z, vp); let _ = driftwm::canvas::camera_for_center(cx, cy, z, vp); });
                    guard!("canvas_visible_fraction", &inp, { let _ = driftwm::canvas::visible_fraction(loc, sz, cam, vp, z); let _ = driftwm::canvas::is_point_visible(cam, cam, vp, z); });
                    guard!("canvas_visible_canvas_rect", &inp, { let _ = driftwm::canvas::visible_canvas_rect(loc, vp, z); });
                    guard!("canvas_bbox_zoom", &inp, {
                        let w = vec![(loc, sz), (Point::from((rng.i(), rng.i())), Size::from((rng.i().saturating_abs(), rng.i().saturating_abs())))];
                        let _ = driftwm::canvas::all_windows_bbox(w.clone().into_iter());
                        let _ = driftwm::canvas::dynamic_min_zoom(w.into_iter(), vp, cx);
                    });
                    guard!("canvas_zoom_to_fit", &inp, { let _ = driftwm::canvas::zoom_to_fit(Rectangle::new(loc, sz), vp, cx); let _ = driftwm::canvas::zoom_anchor_camera(cam, cam, z); let _ = driftwm::canvas::snap_zoom(z); });
                    guard!("canvas_rule_conversion", &inp, { let _ = driftwm::canvas::internal_to_rule(loc, sz); let _ = driftwm::canvas::rule_to_internal(loc.x, loc.y, sz); });
                    guard!("canvas_closest", &inp, { let _ = driftwm::canvas::closest_point_on_rect(cam, loc, sz); });
                    guard!("canvas_coverage", &inp, { let _ = driftwm::canvas::coverage(Rectangle::new(cam, Size::from((sf(cx), sf(cy)))), Rectangle::new(loc, sz)); });
                    // region decomposition
                    let n = rng.below(6);
                    let mut rects = Vec::new();
                    for _ in 0..n {
                        let kind = if rng.below(2) == 0 { smithay::wayland::compositor::RectangleKind::Add } else { smithay::wayland::compositor::RectangleKind::Subtract };
                        rects.push((kind, Rectangle::<i32, Logical>::new(Point::from((rng.i(), rng.i())), Size::from((rng.i().saturating_abs(), rng.i().saturating_abs())))));
                    }
                    guard!("region_decompose", &rects, { let ra = smithay::wayland::compositor::RegionAttributes { rects: rects.clone() }; let mut out = Vec::new(); region::region_to_non_overlapping_rects(&ra, &mut out); });
                    // fill_rect
                    use driftwm::layout::snap::SnapRect;
                    let mk = |r: &mut Rng| SnapRect { x_low: r.f(), x_high: r.f(), y_low: r.f(), y_high: r.f() };
                    let cur = mk(&mut rng); let bnd = mk(&mut rng);
                    let obs: Vec<SnapRect> = (0..rng.below(5)).map(|_| mk(&mut rng)).collect();
                    let (bi, gap) = (rng.f(), rng.f());
                    let mins = (rng.f(), rng.f()); let maxs = (rng.f(), rng.f());
                    guard!("fill_rect", &(cur, bnd, obs.len(), bi, gap, mins, maxs), { let _ = driftwm::layout::fill::fill_rect(cur, &obs, bnd, bi, gap, mins, maxs); });
                    // place_auto
                    use driftwm::layout::auto_placement::{place_auto, Rect};
                    let ws: Vec<Rect> = (0..rng.below(8)).map(|_| Rect { x: rng.f(), y: rng.f(), w: rng.f(), h: rng.f() }).collect();
                    let elig: HashSet<usize> = (0..ws.len()).filter(|_| rng.below(4) != 0).collect();
                    let fi = rng.below(ws.len() + 1);
                    let (nw, nh, vc, g2) = (rng.f(), rng.f(), (rng.f(), rng.f()), rng.f());
                    guard!("place_auto", &(ws.clone(), fi, nw, nh, vc, g2), { let _ = place_auto(&ws, fi, &elig, nw, nh, vc, g2); });
                    // cluster shifts
                    use driftwm::layout::cluster::{resolve_cluster_shifts, ResizeClassification, Side};
                    let sd = |r: &mut Rng| match r.below(5) { 0 => Some(Side::Left), 1 => Some(Side::Right), 2 => Some(Side::Top), 3 => Some(Side::Bottom), _ => None };
                    let members: Vec<ResizeClassification> = (0..rng.below(6)).map(|_| ResizeClassification { axis_x: sd(&mut rng), axis_y: sd(&mut rng), initial_rect: mk(&mut rng) }).collect();
                    let bonds: Vec<(usize, usize)> = (0..rng.below(5)).map(|_| (rng.below(members.len() + 2), rng.below(members.len() + 2))).collect();
                    let prim = if rng.below(2) == 0 { Some((mk(&mut rng), mk(&mut rng))) } else { None };
                    let (wd, hd, g3) = (rng.i(), rng.i(), rng.f());
                    guard!("resolve_cluster_shifts", &(members.len(), bonds.clone(), wd, hd, g3), { let _ = resolve_cluster_shifts(&members, wd, hd, g3, &bonds, prim); });
                }
                _ => { eprintln!("unknown target"); return; }
            }
        }
    }
    println!("target={target} seed={seed} iterations={iters} seconds={secs} distinct_panics={}", panics.len());
    let mut v: Vec<_> = panics.iter().collect(); v.sort_by(|a, b| b.1 .0.cmp(&a.1 .0));
    for (k, (n, _)) in v { println!("  {n:>8}x  {k}"); }
}
