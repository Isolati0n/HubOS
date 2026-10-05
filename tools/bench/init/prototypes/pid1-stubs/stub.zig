// MEASUREMENT STUB, not an init and not for the image (see stub.c). Written for Zig 0.17.0 (main takes Init.Minimal).
const std = @import("std");
const linux = std.os.linux;

pub fn main(init: std.process.Init.Minimal) void {
    var it = init.args.iterate();
    _ = it.skip();
    if (it.next()) |a| {
        if (std.mem.eql(u8, a, "exit")) return;
        if (std.mem.eql(u8, a, "panic")) @panic("stub panic");
    }
    while (true) {
        var status: i32 = 0;
        const r = linux.wait4(-1, &status, 0, null);
        if (@as(isize, @bitCast(r)) < 0) {
            var ts = linux.timespec{ .sec = 3600, .nsec = 0 };
            _ = linux.nanosleep(&ts, null);
        }
    }
}
