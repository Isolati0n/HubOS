//! Runs the shared trace file (embedded as "all.trace", copied here by run.sh) through the Zig core and compares every step.
const std = @import("std");
const core = @import("core.zig");

const text = @embedFile("all.trace");

fn actLine(buf: []u8, a: core.Act) []const u8 {
    const kn = core.kind_names[@intFromEnum(a.kind)];
    return switch (a.t) {
        .deliver => std.fmt.bufPrint(buf, "D {d} {s} {d}", .{ a.dest, kn, a.code }) catch unreachable,
        .deliver_hub => std.fmt.bufPrint(buf, "H {s} {d}", .{ kn, a.code }) catch unreachable,
        .drop => std.fmt.bufPrint(buf, "X", .{}) catch unreachable,
        .flash => std.fmt.bufPrint(buf, "F {d}", .{a.dest}) catch unreachable,
        .synth_ku => std.fmt.bufPrint(buf, "SK {d} {d}", .{ a.dest, a.code }) catch unreachable,
        .synth_bu => std.fmt.bufPrint(buf, "SB {d} {d}", .{ a.dest, a.code }) catch unreachable,
        .release_all => std.fmt.bufPrint(buf, "RA {d}", .{a.dest}) catch unreachable,
    };
}

fn lessStr(_: void, a: []const u8, b: []const u8) bool {
    return std.mem.lessThan(u8, a, b);
}

const Pair = struct { d: u8, c: u32 };
fn lessPair(_: void, a: Pair, b: Pair) bool {
    return if (a.d != b.d) a.d < b.d else a.c < b.c;
}

fn heldText(w: *std.Io.Writer, list: []const core.Held, n: usize) !void {
    var arr: [core.MAX_KEYS]Pair = undefined;
    var i: usize = 0;
    while (i < n) : (i += 1) arr[i] = .{ .d = list[i].dest, .c = list[i].code };
    std.mem.sort(Pair, arr[0..n], {}, lessPair);
    if (n == 0) {
        try w.writeAll("-");
        return;
    }
    i = 0;
    while (i < n) : (i += 1) {
        if (i > 0) try w.writeAll(",");
        try w.print("{d}:{d}", .{ arr[i].d, arr[i].c });
    }
}

fn stateLine(buf: []u8, st: *const core.State, n: usize) ![]const u8 {
    var w = std.Io.Writer.fixed(buf);
    try w.print("state focus={d} epoch={d} waiting=", .{ st.focus, st.epoch });
    if (st.waiting == core.NONE) try w.writeAll("-") else try w.print("{d}", .{st.waiting});
    try w.writeAll(" gen=");
    var m: usize = 1;
    while (m <= n) : (m += 1) {
        if (m > 1) try w.writeAll(",");
        try w.print("{d}", .{st.gen[m]});
    }
    try w.writeAll(" conn=");
    m = 1;
    while (m <= n) : (m += 1) {
        if (m > 1) try w.writeAll(",");
        try w.print("{d}", .{@intFromBool(st.conn[m])});
    }
    try w.writeAll(" owed=");
    m = 1;
    while (m <= n) : (m += 1) {
        if (m > 1) try w.writeAll(",");
        try w.print("{d}", .{@intFromBool(st.owed[m])});
    }
    try w.writeAll(" keys=");
    try heldText(&w, &st.keys, st.nkeys);
    try w.writeAll(" btns=");
    try heldText(&w, &st.btns, st.nbtns);
    return w.buffered();
}

fn parseKind(s: []const u8) ?core.Kind {
    for (core.kind_names, 0..) |n, i| {
        if (std.mem.eql(u8, n, s)) return @enumFromInt(i);
    }
    return null;
}

pub fn main() !void {
    var st = core.State{};
    var out = core.Out{};
    var n: usize = 3;
    var steps: usize = 0;
    var scenarios: usize = 0;
    var bad: usize = 0;
    var pending: ?core.Event = null;
    var want: [core.MAX_ACTS][64]u8 = undefined;
    var want_len: [core.MAX_ACTS]usize = undefined;
    var nwant: usize = 0;
    var name: []const u8 = "";
    var lines = std.mem.splitScalar(u8, text, '\n');
    var ln: usize = 0;
    while (lines.next()) |raw| {
        ln += 1;
        const line = std.mem.trim(u8, raw, " \t\r");
        if (line.len == 0 or line[0] == '#') continue;
        if (std.mem.startsWith(u8, line, "n ")) {
            n = try std.fmt.parseInt(usize, line[2..], 10);
        } else if (std.mem.startsWith(u8, line, "scenario ")) {
            name = line[9..];
            st = core.State{};
            scenarios += 1;
        } else if (std.mem.eql(u8, line, "end")) {
            // nothing
        } else if (std.mem.startsWith(u8, line, "ev ")) {
            var it = std.mem.tokenizeScalar(u8, line, ' ');
            _ = it.next();
            const kind = parseKind(it.next() orelse "") orelse {
                std.debug.print("line {d}: bad event\n", .{ln});
                bad += 1;
                continue;
            };
            const a = try std.fmt.parseInt(u8, it.next().?, 10);
            const code = try std.fmt.parseInt(u32, it.next().?, 10);
            const gen = try std.fmt.parseInt(u32, it.next().?, 10);
            const ep = try std.fmt.parseInt(u32, it.next().?, 10);
            pending = .{ .kind = kind, .a = a, .code = code, .gen = gen, .ep = ep };
            nwant = 0;
        } else if (std.mem.startsWith(u8, line, "act ")) {
            const rest = line[4..];
            @memcpy(want[nwant][0..rest.len], rest);
            want_len[nwant] = rest.len;
            nwant += 1;
        } else if (std.mem.startsWith(u8, line, "state ")) {
            if (pending) |ev| {
                pending = null;
                st.step(ev, &out);
                steps += 1;
                var got: [core.MAX_ACTS][64]u8 = undefined;
                var got_s: [core.MAX_ACTS][]const u8 = undefined;
                var i: usize = 0;
                while (i < out.n) : (i += 1) got_s[i] = actLine(&got[i], out.acts[i]);
                var want_s: [core.MAX_ACTS][]const u8 = undefined;
                i = 0;
                while (i < nwant) : (i += 1) want_s[i] = want[i][0..want_len[i]];
                std.mem.sort([]const u8, got_s[0..out.n], {}, lessStr);
                std.mem.sort([]const u8, want_s[0..nwant], {}, lessStr);
                var same = out.n == nwant;
                if (same) {
                    i = 0;
                    while (i < nwant) : (i += 1) {
                        if (!std.mem.eql(u8, got_s[i], want_s[i])) same = false;
                    }
                }
                if (!same) {
                    std.debug.print("line {d} [{s}] actions differ\n", .{ ln, name });
                    bad += 1;
                }
                var sbuf: [512]u8 = undefined;
                const sl = try stateLine(&sbuf, &st, n);
                if (!std.mem.eql(u8, sl, line)) {
                    std.debug.print("line {d} [{s}] state differs\n  want {s}\n  got  {s}\n", .{ ln, name, line, sl });
                    bad += 1;
                }
            }
        }
    }
    std.debug.print("zig trace: scenarios={d} steps={d} mismatches={d}\n", .{ scenarios, steps, bad });
    if (bad > 0) std.process.exit(1);
}
