//! Fuzz run for the Zig focus core. Same generator and hash as the Rust and Ada candidates.
//! Usage: fuzz SEED CASES. Built with ReleaseSafe (overflow and bounds checks ON): a panic is a failure.
const std = @import("std");
const core = @import("core.zig");

const Rng = struct {
    s: u64,
    fn next(self: *Rng) u64 {
        self.s +%= 0x9E3779B97F4A7C15;
        var z = self.s;
        z = (z ^ (z >> 30)) *% 0xBF58476D1CE4E5B9;
        z = (z ^ (z >> 27)) *% 0x94D049BB133111EB;
        return z ^ (z >> 31);
    }
    fn choose(self: *Rng, n: u32) u32 {
        return @as(u32, @truncate(self.next() >> 32)) % n;
    }
};

const Hash = struct {
    h: u64 = 0xcbf29ce484222325,
    fn feed(self: *Hash, v: u32) void {
        var x = v;
        var i: u32 = 0;
        while (i < 4) : (i += 1) {
            self.h = (self.h ^ @as(u64, x & 255)) *% 0x00000100000001B3;
            x >>= 8;
        }
    }
};

fn satAdd2(x: u32) u32 {
    return x +| 2;
}

fn stamp(r: *Rng, st: *const core.State, d: u8) struct { g: u32, e: u32 } {
    const g: u32 = if (d <= core.MAX_M and r.choose(10) < 8) st.gen[d] else r.choose(3);
    const e: u32 = if (r.choose(10) < 8) st.epoch else r.choose(satAdd2(st.epoch));
    return .{ .g = g, .e = e };
}

fn destOf(r: *Rng, st: *const core.State, nm: u32) u8 {
    if (r.choose(4) < 3) return st.focus;
    return @intCast(r.choose(nm + 1));
}

fn genEvent(r: *Rng, st: *const core.State, nm: u32) core.Event {
    const k = r.choose(100);
    if (k < 50) {
        const sub = r.choose(5);
        var kind: core.Kind = .motion;
        var code: u32 = 0;
        switch (sub) {
            0, 1 => {
                code = if (r.choose(16) == 0) r.choose(64) else r.choose(8);
                kind = if (sub == 0) .kd else .ku;
            },
            2, 3 => {
                kind = if (sub == 2) .bd else .bu;
                code = r.choose(4);
            },
            else => {},
        }
        const d = destOf(r, st, nm);
        const s = stamp(r, st, d);
        return .{ .kind = kind, .a = d, .code = code, .gen = s.g, .ep = s.e };
    }
    if (k < 55) {
        const kind: core.Kind = if (r.choose(2) == 0) .chord else .menu;
        return .{ .kind = kind, .a = core.HUB, .code = 0, .gen = 0, .ep = 0 };
    }
    if (k < 70) {
        const t: u8 = if (r.choose(32) == 0) 200 else @intCast(r.choose(nm + 1));
        return .{ .kind = .req, .a = t, .code = 0, .gen = 0, .ep = 0 };
    }
    if (k < 80) {
        const m: u8 = if (st.waiting != core.NONE and r.choose(10) < 7) st.waiting else @intCast(r.choose(nm) + 1);
        const e: u32 = if (r.choose(10) < 8) st.epoch else r.choose(satAdd2(st.epoch));
        const kind: core.Kind = if (r.choose(2) == 0) .vok else .vto;
        return .{ .kind = kind, .a = m, .code = 0, .gen = 0, .ep = e };
    }
    const m: u8 = @intCast(r.choose(nm) + 1);
    const kind: core.Kind = if (k < 86) .rst else if (k < 93) .dis else .rec;
    return .{ .kind = kind, .a = m, .code = 0, .gen = 0, .ep = 0 };
}

pub fn main(init: std.process.Init) !void {
    var it = init.minimal.args.iterate();
    _ = it.next(); // program name
    const seed: u64 = if (it.next()) |a| try std.fmt.parseInt(u64, a, 10) else 1;
    const cases: u64 = if (it.next()) |a| try std.fmt.parseInt(u64, a, 10) else 1_000_000;
    var rng = Rng{ .s = seed };
    var h = Hash{};
    var steps: u64 = 0;
    const t0 = std.Io.Timestamp.now(init.io, .awake);
    var out = core.Out{};
    var case: u64 = 0;
    while (case < cases) : (case += 1) {
        const nm = 3 + rng.choose(18);
        const len = 8 + rng.choose(25);
        var st = core.State{};
        var ghost = core.Ghost{};
        var step: u32 = 0;
        while (step < len) : (step += 1) {
            const ev = genEvent(&rng, &st, nm);
            const pre = st;
            st.step(ev, &out);
            steps += 1;
            const bad = core.afterStep(&pre, ev, &out, &st, &ghost);
            if (bad != 0) {
                std.debug.print("VIOLATION invariant {d} at case {d} step {d} (seed {d})\n", .{ bad, case, step, seed });
                std.process.exit(1);
            }
            h.feed(@intCast(out.n));
            var i: usize = 0;
            while (i < out.n) : (i += 1) {
                const a = out.acts[i];
                h.feed(@intFromEnum(a.t));
                h.feed(a.dest);
                h.feed(@intFromEnum(a.kind));
                h.feed(a.code);
                h.feed(a.gen);
                h.feed(a.ep);
            }
            h.feed(st.focus);
            h.feed(st.epoch);
            h.feed(st.waiting);
            h.feed(@intCast(st.nkeys));
            h.feed(@intCast(st.nbtns));
            var gsum: u32 = 0;
            var cbits: u32 = 0;
            var m: usize = 1;
            while (m <= core.MAX_M) : (m += 1) {
                gsum +%= st.gen[m];
                cbits += @intFromBool(st.conn[m]);
            }
            h.feed(gsum);
            h.feed(cbits);
        }
    }
    const t1 = std.Io.Timestamp.now(init.io, .awake);
    const ns: i96 = t0.durationTo(t1).nanoseconds;
    const secs: f64 = @as(f64, @floatFromInt(ns)) / 1e9;
    std.debug.print("zig fuzz: seed={d} cases={d} steps={d} violations=0 hash={x:0>16} seconds={d:.2} steps_per_second={d:.0}\n", .{ seed, cases, steps, h.h, secs, @as(f64, @floatFromInt(steps)) / @max(secs, 1e-9) });
}
