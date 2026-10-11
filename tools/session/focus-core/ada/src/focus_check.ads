--  The independent checker of the invariants (docs/proposals/focus-core.md section 3). It sees only the state before,
--  the event, the actions, the state after, and a ghost of what every node believes is held (built only from the
--  emitted actions). Not part of the proven core (SPARK_Mode is off): it is the reference the fuzz run uses.
with Interfaces;
with Focus_Core; use Focus_Core;

package Focus_Check with SPARK_Mode => Off is

   type U64_Array is array (0 .. Max_M) of Interfaces.Unsigned_64;
   type U8_Array is array (0 .. Max_M) of U8;

   type Ghost is record
      Keys : U64_Array := (others => 0);
      Btns : U8_Array := (others => 0);
   end record;

   procedure Reset (G : out Ghost);

   --  0 if every invariant holds, else the number of the first that fails (1 stale delivery, 2 destinations,
   --  3 stuck keys, 4 epoch, 5 hub path, 6 verification rules, 7 capacity).
   function After_Step (Pre : State; Ev : Event; Output : Out_Buf; Post : State; G : in out Ghost) return Natural;

end Focus_Check;
