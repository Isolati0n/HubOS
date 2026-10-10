//! The focus core of the session layer (docs/proposals/focus-core.md): step(state, event) -> (new_state, actions).
//!
//! Pure: no I/O, no heap, no garbage collector, fixed-size arrays only. `State::step` takes the state by mutable
//! reference and writes the actions into a caller-supplied fixed buffer; that is the pure function
//! (state, event) -> (state', actions) in the form that needs no allocation.
//!
//! Machines are numbered 1..=MAX_M, the hub is 0 and "nobody" is 255. The rules are those of
//! docs/proposals/focus-core.md section 2.5 and docs/models/focus-core/FocusCore.tla.
#![no_std]

pub const MAX_M: usize = 20;
/// Placeholder limits (owner question 1 in docs/proposals/focus-core.md).
pub const MAX_KEYS: usize = 32;
pub const MAX_BTNS: usize = 8;
/// A focus change can emit one release per held key and button; one more for a reconnect.
pub const MAX_ACTS: usize = 64;
pub const HUB: u8 = 0;
pub const NONE: u8 = 255;

#[derive(Clone, Copy, PartialEq, Eq, Debug)]
#[repr(u8)]
pub enum Kind {
    Kd = 0,
    Ku = 1,
    Bd = 2,
    Bu = 3,
    Motion = 4,
    Chord = 5,
    Menu = 6,
    Req = 7,
    Vok = 8,
    Vto = 9,
    Rst = 10,
    Dis = 11,
    Rec = 12,
}

impl Kind {
    pub const ALL: [Kind; 13] = [
        Kind::Kd, Kind::Ku, Kind::Bd, Kind::Bu, Kind::Motion, Kind::Chord, Kind::Menu,
        Kind::Req, Kind::Vok, Kind::Vto, Kind::Rst, Kind::Dis, Kind::Rec,
    ];
    pub const NAMES: [&'static str; 13] = [
        "kd", "ku", "bd", "bu", "motion", "chord", "menu", "req", "vok", "vto", "rst", "dis", "rec",
    ];
    pub fn from_u8(n: u8) -> Option<Kind> {
        if (n as usize) < Self::ALL.len() { Some(Self::ALL[n as usize]) } else { None }
    }
    pub fn is_input(self) -> bool {
        (self as u8) <= (Kind::Menu as u8)
    }
}

/// One event. `a` is the destination of an input, the target of a focus request, or the machine of the other events.
/// `gen` and `ep` are the stamps of an input, or the epoch (`ep`) of a verification.
#[derive(Clone, Copy, PartialEq, Eq, Debug)]
pub struct Event {
    pub kind: Kind,
    pub a: u8,
    pub code: u32,
    pub gen: u32,
    pub ep: u32,
}

#[derive(Clone, Copy, PartialEq, Eq, Debug)]
#[repr(u8)]
pub enum ActKind {
    Deliver = 0,
    DeliverHub = 1,
    Drop = 2,
    Flash = 3,
    SynthKu = 4,
    SynthBu = 5,
    ReleaseAll = 6,
}

/// One action. For Deliver: dest, the input kind and code, and the stamps it matched. DeliverHub: kind and code.
/// Flash and ReleaseAll: dest. SynthKu and SynthBu: dest and code.
#[derive(Clone, Copy, PartialEq, Eq, Debug)]
pub struct Act {
    pub t: ActKind,
    pub dest: u8,
    pub kind: Kind,
    pub code: u32,
    pub gen: u32,
    pub ep: u32,
}

pub const NO_ACT: Act = Act { t: ActKind::Drop, dest: 0, kind: Kind::Motion, code: 0, gen: 0, ep: 0 };

pub struct Out {
    pub acts: [Act; MAX_ACTS],
    pub n: usize,
}

impl Out {
    pub const fn new() -> Out {
        Out { acts: [NO_ACT; MAX_ACTS], n: 0 }
    }
    pub fn clear(&mut self) {
        self.n = 0;
    }
    #[inline]
    fn push(&mut self, a: Act) {
        // Cannot overflow: a step emits at most MAX_KEYS + MAX_BTNS + 1 actions (checked by the proof and the fuzz run).
        if self.n < MAX_ACTS {
            self.acts[self.n] = a;
            self.n += 1;
        }
    }
}

#[derive(Clone, Copy, PartialEq, Eq, Debug)]
pub struct Held {
    pub code: u32,
    pub dest: u8,
}

const NO_HELD: Held = Held { code: 0, dest: 0 };

#[derive(Clone, Copy)]
pub struct State {
    pub focus: u8,
    pub epoch: u32,
    pub gen: [u32; MAX_M + 1],
    pub conn: [bool; MAX_M + 1],
    pub waiting: u8,
    pub owed: [bool; MAX_M + 1],
    pub keys: [Held; MAX_KEYS],
    pub nkeys: usize,
    pub btns: [Held; MAX_BTNS],
    pub nbtns: usize,
}

#[inline]
fn is_machine(m: u8) -> bool {
    m >= 1 && (m as usize) <= MAX_M
}

fn find(list: &[Held], n: usize, code: u32, dest: u8) -> Option<usize> {
    let mut i = 0;
    while i < n {
        if list[i].code == code && list[i].dest == dest {
            return Some(i);
        }
        i += 1;
    }
    None
}

/// Remove entry i, keeping the order of the rest.
fn remove_at(list: &mut [Held], n: &mut usize, i: usize) {
    let mut j = i;
    while j + 1 < *n {
        list[j] = list[j + 1];
        j += 1;
    }
    *n -= 1;
}

/// Remove every entry whose destination is `dest`.
fn remove_dest(list: &mut [Held], n: &mut usize, dest: u8) {
    let mut w = 0;
    let mut r = 0;
    while r < *n {
        if list[r].dest != dest {
            list[w] = list[r];
            w += 1;
        }
        r += 1;
    }
    *n = w;
}

fn count_dest(list: &[Held], n: usize, dest: u8) -> usize {
    let mut c = 0;
    let mut i = 0;
    while i < n {
        if list[i].dest == dest {
            c += 1;
        }
        i += 1;
    }
    c
}

impl State {
    pub const fn new() -> State {
        State {
            focus: HUB,
            epoch: 0,
            gen: [0; MAX_M + 1],
            conn: [true; MAX_M + 1],
            waiting: NONE,
            owed: [false; MAX_M + 1],
            keys: [NO_HELD; MAX_KEYS],
            nkeys: 0,
            btns: [NO_HELD; MAX_BTNS],
            nbtns: 0,
        }
    }

    /// Apply one event. The actions are written to `out` (cleared first).
    pub fn step(&mut self, ev: &Event, out: &mut Out) {
        out.clear();
        match ev.kind {
            Kind::Kd | Kind::Ku | Kind::Bd | Kind::Bu | Kind::Motion => self.input(ev, out),
            Kind::Chord | Kind::Menu => {
                // The hub path: never subject to the delivery rule, in every state.
                out.push(Act { t: ActKind::DeliverHub, dest: HUB, kind: ev.kind, code: ev.code, gen: 0, ep: 0 });
            }
            Kind::Req => self.request(ev.a, out),
            Kind::Vok => {
                let m = ev.a;
                if is_machine(m) && self.waiting == m && ev.ep == self.epoch && self.conn[m as usize] {
                    self.focus = m;
                    self.waiting = NONE;
                }
            }
            Kind::Vto => {
                let m = ev.a;
                if is_machine(m) && self.waiting == m && ev.ep == self.epoch {
                    self.focus = HUB;
                    self.waiting = NONE;
                }
            }
            Kind::Rst => self.restart(ev.a),
            Kind::Dis => {
                let m = ev.a;
                if is_machine(m) && self.conn[m as usize] {
                    self.conn[m as usize] = false;
                    if self.waiting == m {
                        self.waiting = NONE;
                    }
                }
            }
            Kind::Rec => {
                let m = ev.a;
                if is_machine(m) && !self.conn[m as usize] {
                    self.conn[m as usize] = true;
                    remove_dest(&mut self.keys, &mut self.nkeys, m);
                    remove_dest(&mut self.btns, &mut self.nbtns, m);
                    self.owed[m as usize] = false;
                    out.push(Act { t: ActKind::ReleaseAll, dest: m, kind: Kind::Motion, code: 0, gen: 0, ep: 0 });
                }
            }
        }
    }

    fn input(&mut self, ev: &Event, out: &mut Out) {
        let d = ev.a;
        let ok = is_machine(d)
            && d == self.focus
            && ev.gen == self.gen[d as usize]
            && ev.ep == self.epoch
            && self.conn[d as usize];
        if ok {
            // Held-key bookkeeping first, so that a press that does not fit is dropped, not delivered.
            match ev.kind {
                Kind::Kd => {
                    if find(&self.keys, self.nkeys, ev.code, d).is_none() {
                        if self.nkeys >= MAX_KEYS {
                            out.push(Act { t: ActKind::Drop, dest: d, kind: ev.kind, code: ev.code, gen: 0, ep: 0 });
                            return;
                        }
                        self.keys[self.nkeys] = Held { code: ev.code, dest: d };
                        self.nkeys += 1;
                    }
                }
                Kind::Ku => {
                    if let Some(i) = find(&self.keys, self.nkeys, ev.code, d) {
                        remove_at(&mut self.keys, &mut self.nkeys, i);
                    }
                }
                Kind::Bd => {
                    if find(&self.btns, self.nbtns, ev.code, d).is_none() {
                        if self.nbtns >= MAX_BTNS {
                            out.push(Act { t: ActKind::Drop, dest: d, kind: ev.kind, code: ev.code, gen: 0, ep: 0 });
                            return;
                        }
                        self.btns[self.nbtns] = Held { code: ev.code, dest: d };
                        self.nbtns += 1;
                    }
                }
                Kind::Bu => {
                    if let Some(i) = find(&self.btns, self.nbtns, ev.code, d) {
                        remove_at(&mut self.btns, &mut self.nbtns, i);
                    }
                }
                _ => {}
            }
            out.push(Act { t: ActKind::Deliver, dest: d, kind: ev.kind, code: ev.code, gen: ev.gen, ep: ev.ep });
        } else if d == HUB && self.focus == HUB && ev.ep == self.epoch {
            out.push(Act { t: ActKind::DeliverHub, dest: HUB, kind: ev.kind, code: ev.code, gen: 0, ep: 0 });
        } else {
            out.push(Act { t: ActKind::Drop, dest: d, kind: ev.kind, code: ev.code, gen: 0, ep: 0 });
            if is_machine(d) && !self.conn[d as usize] {
                out.push(Act { t: ActKind::Flash, dest: d, kind: ev.kind, code: 0, gen: 0, ep: 0 });
            }
        }
    }

    /// Focus change to `t`: the epoch is bumped FIRST.
    fn request(&mut self, t: u8, out: &mut Out) {
        if t != HUB && !is_machine(t) {
            return; // not a destination: ignored
        }
        self.epoch = self.epoch.saturating_add(1);
        let old = self.focus;
        if is_machine(old) {
            let oldk = count_dest(&self.keys, self.nkeys, old);
            let oldb = count_dest(&self.btns, self.nbtns, old);
            if self.conn[old as usize] {
                let mut i = 0;
                while i < self.nkeys {
                    if self.keys[i].dest == old {
                        out.push(Act { t: ActKind::SynthKu, dest: old, kind: Kind::Ku, code: self.keys[i].code, gen: 0, ep: 0 });
                    }
                    i += 1;
                }
                let mut i = 0;
                while i < self.nbtns {
                    if self.btns[i].dest == old {
                        out.push(Act { t: ActKind::SynthBu, dest: old, kind: Kind::Bu, code: self.btns[i].code, gen: 0, ep: 0 });
                    }
                    i += 1;
                }
            } else if oldk + oldb > 0 {
                self.owed[old as usize] = true; // sent at reconnect
            }
        }
        self.nkeys = 0;
        self.nbtns = 0;
        self.focus = HUB;
        self.waiting = if t == HUB { NONE } else { t };
    }

    fn restart(&mut self, m: u8) {
        if !is_machine(m) {
            return;
        }
        let mi = m as usize;
        let was_focus = self.focus == m;
        let was_waiting = self.waiting == m;
        self.gen[mi] = self.gen[mi].saturating_add(1);
        self.conn[mi] = false;
        remove_dest(&mut self.keys, &mut self.nkeys, m);
        remove_dest(&mut self.btns, &mut self.nbtns, m);
        self.owed[mi] = true;
        if was_focus || was_waiting {
            self.epoch = self.epoch.saturating_add(1);
            if was_focus {
                self.focus = HUB;
            }
            self.waiting = m; // return only after verification
        }
    }
}

impl Default for State {
    fn default() -> Self {
        State::new()
    }
}

/// The independent checker of the invariants (docs/proposals/focus-core.md section 3). It sees only the state
/// before, the event, the actions and the state after, and a ghost of what every node believes is held.
/// The same checker is used by the fuzz run and the Kani proofs.
pub mod check {
    use super::*;

    /// What each node believes is held, built ONLY from the emitted actions. Keys 0..64 and buttons 0..8 (the
    /// fuzz and proof inputs stay inside that range).
    #[derive(Clone, Copy)]
    pub struct Ghost {
        pub keys: [u64; MAX_M + 1],
        pub btns: [u8; MAX_M + 1],
    }

    impl Ghost {
        pub const fn new() -> Ghost {
            Ghost { keys: [0; MAX_M + 1], btns: [0; MAX_M + 1] }
        }
        pub fn apply(&mut self, out: &Out) {
            let mut i = 0;
            while i < out.n {
                let a = out.acts[i];
                let d = a.dest as usize;
                match a.t {
                    ActKind::Deliver => match a.kind {
                        Kind::Kd => self.keys[d] |= 1u64 << (a.code & 63),
                        Kind::Ku => self.keys[d] &= !(1u64 << (a.code & 63)),
                        Kind::Bd => self.btns[d] |= 1u8 << (a.code & 7),
                        Kind::Bu => self.btns[d] &= !(1u8 << (a.code & 7)),
                        _ => {}
                    },
                    ActKind::SynthKu => self.keys[d] &= !(1u64 << (a.code & 63)),
                    ActKind::SynthBu => self.btns[d] &= !(1u8 << (a.code & 7)),
                    ActKind::ReleaseAll => {
                        self.keys[d] = 0;
                        self.btns[d] = 0;
                    }
                    _ => {}
                }
                i += 1;
            }
        }
    }

    /// Returns 0 if every invariant holds, else the number of the first one that fails
    /// (1 stale delivery, 2 destinations, 3 stuck keys, 4 epoch, 5 hub path, 6 verification rules, 7 capacity).
    pub fn after_step(pre: &State, ev: &Event, out: &Out, post: &State, ghost: &mut Ghost) -> u8 {
        if out.n > MAX_ACTS || post.nkeys > MAX_KEYS || post.nbtns > MAX_BTNS {
            return 7;
        }
        // 1 and 2
        let mut delivers = 0;
        let mut i = 0;
        while i < out.n {
            let a = out.acts[i];
            match a.t {
                ActKind::Deliver => {
                    delivers += 1;
                    let d = a.dest as usize;
                    if !(is_machine(a.dest) && a.dest == pre.focus && a.gen == pre.gen[d] && a.ep == pre.epoch && pre.conn[d]) {
                        return 1;
                    }
                }
                ActKind::DeliverHub => delivers += 1,
                _ => {}
            }
            i += 1;
        }
        if delivers > 1 {
            return 2;
        }
        // 4
        if post.epoch < pre.epoch {
            return 4;
        }
        // 5
        if matches!(ev.kind, Kind::Chord | Kind::Menu) {
            let mut found = false;
            let mut i = 0;
            while i < out.n {
                if out.acts[i].t == ActKind::DeliverHub && out.acts[i].kind == ev.kind {
                    found = true;
                }
                i += 1;
            }
            if !found {
                return 5;
            }
        }
        // 6: verification rules
        let valid_v = matches!(ev.kind, Kind::Vok | Kind::Vto)
            && is_machine(ev.a)
            && pre.waiting == ev.a
            && ev.ep == pre.epoch
            && (ev.kind == Kind::Vto || pre.conn[ev.a as usize]);
        if matches!(ev.kind, Kind::Vok | Kind::Vto) && !valid_v && (post.focus != pre.focus || post.waiting != pre.waiting) {
            return 6;
        }
        if ev.kind == Kind::Vto && valid_v && !(post.waiting == NONE && post.focus == HUB) {
            return 6;
        }
        if post.waiting != NONE && post.focus != HUB {
            return 6;
        }
        // 3: only the focus machine holds anything; and what a node believes equals what the core holds, unless a release is owed
        ghost.apply(out);
        let mut m = 1;
        while m <= MAX_M {
            let mk = count_dest(&post.keys, post.nkeys, m as u8);
            let mb = count_dest(&post.btns, post.nbtns, m as u8);
            if (m as u8) != post.focus && (mk > 0 || mb > 0) {
                return 3;
            }
            if !post.owed[m] {
                let mut bk: u64 = 0;
                let mut i = 0;
                while i < post.nkeys {
                    if post.keys[i].dest == m as u8 {
                        bk |= 1u64 << (post.keys[i].code & 63);
                    }
                    i += 1;
                }
                let mut bb: u8 = 0;
                let mut i = 0;
                while i < post.nbtns {
                    if post.btns[i].dest == m as u8 {
                        bb |= 1u8 << (post.btns[i].code & 7);
                    }
                    i += 1;
                }
                if ghost.keys[m] != bk || ghost.btns[m] != bb {
                    return 3;
                }
            }
            m += 1;
        }
        0
    }
}
