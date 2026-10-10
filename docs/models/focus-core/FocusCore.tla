---------------------------- MODULE FocusCore ----------------------------
(* TLA+ model of the focus core of the session layer: step(state, event) -> (new_state, actions).
   docs/proposals/focus-core.md says what the rules are; this file states them for TLC.
   Machines are 1..N, the hub is 0, "nobody" is -1. Keys and buttons are small numbers.
   The model does not keep the last event or the last actions as state (that would multiply the state
   space); instead every step checks its own actions against the invariants and sets the flag `bad`
   when one fails. TLC then checks that `bad` stays FALSE. *)
EXTENDS Naturals, Integers, FiniteSets, TLC

CONSTANTS N,          \* number of machines (3 in the checked model)
          NKeys,      \* number of distinct keys (2)
          NButtons,   \* number of distinct mouse buttons (1)
          MaxEpoch,   \* the model bumps the epoch at most this often (a bound of the MODEL; the real core saturates)
          MaxGen,     \* each worker restarts at most this often (bound of the MODEL)
          T           \* the verification timeout, in model ticks (stands for the real VERIFY_TIMEOUT)

HUB  == 0
NONE == -1
M    == 1..N
Dest == M \cup {HUB}
Keys == 1..NKeys
Btns == 1..NButtons

VARIABLES focus, epoch, gen, conn, held, heldB, waiting, owed, nodeK, nodeB, age, bad
vars == <<focus, epoch, gen, conn, held, heldB, waiting, owed, nodeK, nodeB, age, bad>>

(* ---------------- events ---------------- *)
InputKinds == {"kd", "ku", "bd", "bu", "chord", "menu"}

KeyInputs == { [kind |-> k, code |-> c, dest |-> d, gen |-> g, ep |-> e] :
               k \in {"kd", "ku"}, c \in Keys, d \in Dest, g \in 0..MaxGen, e \in 0..MaxEpoch }
BtnInputs == { [kind |-> k, code |-> c, dest |-> d, gen |-> g, ep |-> e] :
               k \in {"bd", "bu"}, c \in Btns, d \in Dest, g \in 0..MaxGen, e \in 0..MaxEpoch }
Specials  == { [kind |-> k, code |-> 0, dest |-> HUB, gen |-> 0, ep |-> 0] : k \in {"chord", "menu"} }
Reqs      == { [kind |-> "req", target |-> t] : t \in Dest }
Verifs    == { [kind |-> k, m |-> m, ep |-> e] : k \in {"vok", "vto"}, m \in M, e \in 0..MaxEpoch }
PerM      == { [kind |-> k, m |-> m] : k \in {"rst", "dis", "rec"}, m \in M }
Ticks     == { [kind |-> "tick"] }
Events    == KeyInputs \cup BtnInputs \cup Specials \cup Reqs \cup Verifs \cup PerM \cup Ticks

(* ---------------- the ghost: what each node believes is held ---------------- *)
\* Actions are records. Within one step there is never an add and a remove for the same machine,
\* so the order inside the set does not matter.
UpdK(s, m, acts) ==
  LET adds == { a.code : a \in { x \in acts : x.t = "Deliver" /\ x.dest = m /\ x.kind = "kd" } }
      rems == { a.code : a \in { x \in acts : (x.t = "Deliver" /\ x.dest = m /\ x.kind = "ku") \/ (x.t = "SynthKU" /\ x.dest = m) } }
      all  == \E a \in acts : a.t = "RelAll" /\ a.dest = m
  IN IF all THEN {} ELSE (s \cup adds) \ rems
UpdB(s, m, acts) ==
  LET adds == { a.code : a \in { x \in acts : x.t = "Deliver" /\ x.dest = m /\ x.kind = "bd" } }
      rems == { a.code : a \in { x \in acts : (x.t = "Deliver" /\ x.dest = m /\ x.kind = "bu") \/ (x.t = "SynthBU" /\ x.dest = m) } }
      all  == \E a \in acts : a.t = "RelAll" /\ a.dest = m
  IN IF all THEN {} ELSE (s \cup adds) \ rems

(* ---------------- the properties checked on every step (pre-state unprimed, post-state primed) ---------------- *)
Valid(ev) ==   \* a verification event that counts
  CASE ev.kind = "vok" -> waiting = ev.m /\ ev.ep = epoch /\ conn[ev.m]
    [] ev.kind = "vto" -> waiting = ev.m /\ ev.ep = epoch /\ age >= T
    [] OTHER -> FALSE

StepProps(acts, ev) ==
  /\ \* 1. No stale delivery: every Deliver matched focus, generation, epoch and connection of the PRE-state.
     \A a \in acts : a.t = "Deliver" => (a.dest = focus /\ a.g = gen[a.dest] /\ a.ep = epoch /\ conn[a.dest])
  /\ \* 2. At most one destination, and a node destination is the focus.
     Cardinality({ a \in acts : a.t \in {"Deliver", "DeliverHub"} }) <= 1
  /\ \* 4. The epoch never decreases.
     epoch' >= epoch
  /\ \* 5. The flip chord and the menu key reach the hub in every state.
     (ev.kind \in {"chord", "menu"} => \E a \in acts : a.t = "DeliverHub" /\ a.kind = ev.kind)
  /\ \* A valid timeout falls back to the hub and ends the wait.
     (ev.kind = "vto" /\ Valid(ev) => (waiting' = NONE /\ focus' = HUB))
  /\ \* A stale verification (wrong epoch, wrong machine, no longer waiting) never moves the focus.
     (ev.kind \in {"vok", "vto"} /\ ~Valid(ev) => focus' = focus /\ waiting' = waiting)
  /\ \* Nothing is delivered to a node that is waiting for verification: the focus is the hub then.
     (waiting' # NONE => focus' = HUB)

Finish(acts, ev) ==
  /\ nodeK' = [m \in M |-> UpdK(nodeK[m], m, acts)]
  /\ nodeB' = [m \in M |-> UpdB(nodeB[m], m, acts)]
  /\ bad' = (bad \/ ~StepProps(acts, ev))

(* ---------------- steps ---------------- *)
InputStep(ev) ==
  LET d       == ev.dest
      special == ev.kind \in {"chord", "menu"}
      ok      == d \in M /\ d = focus /\ ev.gen = gen[d] /\ ev.ep = epoch /\ conn[d]
      hubok   == d = HUB /\ focus = HUB /\ ev.ep = epoch
      acts    == IF special THEN { [t |-> "DeliverHub", kind |-> ev.kind, code |-> ev.code] }
                 ELSE IF ok THEN { [t |-> "Deliver", dest |-> d, kind |-> ev.kind, code |-> ev.code, g |-> ev.gen, ep |-> ev.ep] }
                 ELSE IF hubok THEN { [t |-> "DeliverHub", kind |-> ev.kind, code |-> ev.code] }
                 ELSE { [t |-> "Drop"] } \cup (IF d \in M /\ ~conn[d] THEN { [t |-> "Flash", dest |-> d] } ELSE {})
  IN /\ IF ok /\ ev.kind = "kd" THEN held' = [held EXCEPT ![d] = @ \cup {ev.code}]
        ELSE IF ok /\ ev.kind = "ku" THEN held' = [held EXCEPT ![d] = @ \ {ev.code}]
        ELSE held' = held
     /\ IF ok /\ ev.kind = "bd" THEN heldB' = [heldB EXCEPT ![d] = @ \cup {ev.code}]
        ELSE IF ok /\ ev.kind = "bu" THEN heldB' = [heldB EXCEPT ![d] = @ \ {ev.code}]
        ELSE heldB' = heldB
     /\ UNCHANGED <<focus, epoch, gen, conn, waiting, owed, age>>
     /\ Finish(acts, ev)

ReqStep(ev) ==   \* focus change: the epoch is bumped FIRST
  LET t     == ev.target
      old   == focus
      oldM  == old \in M
      hk    == IF oldM THEN held[old] ELSE {}
      hb    == IF oldM THEN heldB[old] ELSE {}
      send  == oldM /\ conn[old]
      acts  == IF send THEN { [t |-> "SynthKU", dest |-> old, code |-> k] : k \in hk }
                            \cup { [t |-> "SynthBU", dest |-> old, code |-> b] : b \in hb }
               ELSE {}
  IN /\ epoch < MaxEpoch
     /\ epoch' = epoch + 1
     /\ held' = [m \in M |-> {}]
     /\ heldB' = [m \in M |-> {}]
     /\ owed' = [m \in M |-> IF oldM /\ m = old /\ ~conn[old] /\ (hk # {} \/ hb # {}) THEN TRUE ELSE owed[m]]
     /\ focus' = HUB
     /\ waiting' = IF t = HUB THEN NONE ELSE t
     /\ age' = 0
     /\ UNCHANGED <<gen, conn>>
     /\ Finish(acts, ev)

VerifStep(ev) ==
  /\ IF Valid(ev)
        THEN IF ev.kind = "vok"
                THEN /\ focus' = ev.m /\ waiting' = NONE /\ age' = 0
                ELSE /\ focus' = HUB /\ waiting' = NONE /\ age' = 0
        ELSE /\ UNCHANGED <<focus, waiting, age>>
  /\ UNCHANGED <<epoch, gen, conn, held, heldB, owed>>
  /\ Finish({}, ev)

RstStep(ev) ==   \* worker restart of m
  LET m == ev.m
      wasF == focus = m
      wasW == waiting = m
  IN /\ gen[m] < MaxGen
     /\ (wasF \/ wasW) => epoch < MaxEpoch
     /\ gen' = [gen EXCEPT ![m] = @ + 1]
     /\ conn' = [conn EXCEPT ![m] = FALSE]
     /\ held' = [held EXCEPT ![m] = {}]
     /\ heldB' = [heldB EXCEPT ![m] = {}]
     /\ owed' = [owed EXCEPT ![m] = TRUE]
     /\ IF wasF \/ wasW
           THEN /\ epoch' = epoch + 1
                /\ focus' = IF wasF THEN HUB ELSE focus
                /\ waiting' = m
                /\ age' = 0
           ELSE UNCHANGED <<epoch, focus, waiting, age>>
     /\ Finish({}, ev)

DisStep(ev) ==
  LET m == ev.m IN
  /\ conn[m]
  /\ conn' = [conn EXCEPT ![m] = FALSE]
  /\ IF waiting = m THEN /\ waiting' = NONE /\ age' = 0 ELSE UNCHANGED <<waiting, age>>
  /\ UNCHANGED <<focus, epoch, gen, held, heldB, owed>>
  /\ Finish({}, ev)

RecStep(ev) ==
  LET m == ev.m IN
  /\ ~conn[m]
  /\ conn' = [conn EXCEPT ![m] = TRUE]
  /\ held' = [held EXCEPT ![m] = {}]
  /\ heldB' = [heldB EXCEPT ![m] = {}]
  /\ owed' = [owed EXCEPT ![m] = FALSE]
  /\ UNCHANGED <<focus, epoch, gen, waiting, age>>
  /\ Finish({ [t |-> "RelAll", dest |-> m] }, ev)

TickStep(ev) ==
  /\ waiting # NONE /\ age < T
  /\ age' = age + 1
  /\ UNCHANGED <<focus, epoch, gen, conn, held, heldB, waiting, owed>>
  /\ Finish({}, ev)

Step(ev) ==
  CASE ev.kind \in InputKinds -> InputStep(ev)
    [] ev.kind = "req"        -> ReqStep(ev)
    [] ev.kind \in {"vok", "vto"} -> VerifStep(ev)
    [] ev.kind = "rst"        -> RstStep(ev)
    [] ev.kind = "dis"        -> DisStep(ev)
    [] ev.kind = "rec"        -> RecStep(ev)
    [] ev.kind = "tick"       -> TickStep(ev)

\* The model clock is urgent: when the verification is overdue (age = T), only events that end or restart
\* the wait are possible. This is an ASSUMPTION about the I/O shell: it delivers the timeout when its timer fires.
Urgent == waiting # NONE /\ age = T
UrgentOK(ev) ==
  CASE ev.kind = "vok" -> ev.m = waiting /\ ev.ep = epoch /\ conn[waiting]
    [] ev.kind = "vto" -> ev.m = waiting /\ ev.ep = epoch
    [] ev.kind = "dis" -> ev.m = waiting
    [] ev.kind = "rst" -> ev.m = waiting
    [] ev.kind = "req" -> TRUE
    [] OTHER -> FALSE

Init ==
  /\ focus = HUB /\ epoch = 0
  /\ gen = [m \in M |-> 0] /\ conn = [m \in M |-> TRUE]
  /\ held = [m \in M |-> {}] /\ heldB = [m \in M |-> {}]
  /\ waiting = NONE /\ owed = [m \in M |-> FALSE]
  /\ nodeK = [m \in M |-> {}] /\ nodeB = [m \in M |-> {}]
  /\ age = 0 /\ bad = FALSE

Next == \E ev \in Events : (Urgent => UrgentOK(ev)) /\ Step(ev)

Spec == Init /\ [][Next]_vars

(* ---------------- state invariants ---------------- *)
TypeOK ==
  /\ focus \in Dest /\ epoch \in 0..MaxEpoch
  /\ gen \in [M -> 0..MaxGen] /\ conn \in [M -> BOOLEAN]
  /\ held \in [M -> SUBSET Keys] /\ heldB \in [M -> SUBSET Btns]
  /\ waiting \in Dest \cup {NONE} /\ owed \in [M -> BOOLEAN]
  /\ nodeK \in [M -> SUBSET Keys] /\ nodeB \in [M -> SUBSET Btns]
  /\ age \in 0..T /\ bad \in BOOLEAN

NoBad == ~bad     \* invariants 1, 2, 4, 5 and the verification rules (checked inside every step)

\* Invariant 3: no stuck keys. Only the focus machine holds anything; and unless a release is owed to a machine,
\* what that machine believes is held (the ghost, built only from the emitted actions) is exactly what the core holds.
NoStuckKeys ==
  /\ \A m \in M : m # focus => (held[m] = {} /\ heldB[m] = {})
  /\ \A m \in M : ~owed[m] => (nodeK[m] = held[m] /\ nodeB[m] = heldB[m])

\* Invariant 6 (bounded transition, under the urgent timeout): a wait never lasts more than T ticks,
\* and while waiting the focus is the hub.
Bounded == (waiting # NONE => (age <= T /\ focus = HUB))
=============================================================================
