---------------------------- MODULE FocusCoreRun ----------------------------
(* Runs ONE scripted sequence of events through the model's Step relation. Used to make the shared trace suite:
   the model is the oracle, the script only chooses the events. A generated module defines ScriptDef (a sequence
   of event records); the .cfg overrides Script with it and asks TLC to "violate" NotDone, which makes TLC print
   the whole run as a trace. *)
EXTENDS FocusCore
CONSTANT Script
VARIABLE pc
rvars == <<vars, pc>>
RInit == Init /\ pc = 0
RNext == /\ pc < Len(Script)
         /\ pc' = pc + 1
         /\ Step(Script[pc + 1])
RSpec == RInit /\ [][RNext]_rvars
NotDone == pc < Len(Script)
=============================================================================
