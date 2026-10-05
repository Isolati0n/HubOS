// MEASUREMENT STUB, not an init and not for the image (see stub.c).
use std::time::Duration;

extern "C" {
    fn waitpid(pid: i32, status: *mut i32, options: i32) -> i32;
}

fn main() {
    let a: Vec<String> = std::env::args().collect();
    if a.len() > 1 && a[1] == "exit" {
        return;
    }
    if a.len() > 1 && a[1] == "panic" {
        let v: Vec<u8> = Vec::new();
        let i = a.len() + 5;
        println!("{}", v[i]); // index out of range: a Rust panic
    }
    loop {
        if unsafe { waitpid(-1, std::ptr::null_mut(), 0) } < 0 {
            std::thread::sleep(Duration::from_secs(3600));
        }
    }
}
