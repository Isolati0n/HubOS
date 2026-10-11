--  Fuzz run for the Ada focus core. Same generator, same hash as the Rust and Zig candidates.
--  Usage: fuzz_run SEED CASES
with Ada.Text_IO; use Ada.Text_IO;
with Ada.Command_Line;
with Ada.Calendar;
with Interfaces; use Interfaces;
with Focus_Core; use Focus_Core;
with Focus_Check;

procedure Fuzz_Run is

   Rng_State : Unsigned_64 := 1;

   function Next return Unsigned_64 is
      Z : Unsigned_64;
   begin
      Rng_State := Rng_State + 16#9E37_79B9_7F4A_7C15#;
      Z := Rng_State;
      Z := (Z xor Shift_Right (Z, 30)) * 16#BF58_476D_1CE4_E5B9#;
      Z := (Z xor Shift_Right (Z, 27)) * 16#94D0_49BB_1331_11EB#;
      return Z xor Shift_Right (Z, 31);
   end Next;

   function Choose (N : U32) return U32 is (U32 (Shift_Right (Next, 32)) mod N);

   Hash : Unsigned_64 := 16#CBF2_9CE4_8422_2325#;

   procedure Feed (V : U32) is
      X : U32 := V;
   begin
      for I in 1 .. 4 loop
         Hash := (Hash xor Unsigned_64 (X and 255)) * 16#0000_0100_0000_01B3#;
         X := Shift_Right (X, 8);
      end loop;
   end Feed;

   function Sat_Add2 (X : U32) return U32 is (if X >= U32'Last - 1 then U32'Last else X + 2);

   procedure Stamp (St : State; D : U8; G : out U32; E : out U32) is
   begin
      if Natural (D) <= Max_M and then Choose (10) < 8 then
         G := St.Gen (Integer (D));
      else
         G := Choose (3);
      end if;
      if Choose (10) < 8 then
         E := St.Epoch;
      else
         E := Choose (Sat_Add2 (St.Epoch));
      end if;
   end Stamp;

   function Dest (St : State; Nm : U32) return U8 is
   begin
      if Choose (4) < 3 then
         return St.Focus;
      else
         return U8 (Choose (Nm + 1));
      end if;
   end Dest;

   function Gen_Event (St : State; Nm : U32) return Event is
      K : constant U32 := Choose (100);
      Ev : Event;
   begin
      if K < 50 then
         declare
            Sub  : constant U32 := Choose (5);
            Kd_K : Kind;
            Code : U32;
            D    : U8;
            G, E : U32;
         begin
            case Sub is
               when 0 | 1 =>
                  if Choose (16) = 0 then
                     Code := Choose (64);
                  else
                     Code := Choose (8);
                  end if;
                  Kd_K := (if Sub = 0 then Kd else Ku);
               when 2 | 3 =>
                  Kd_K := (if Sub = 2 then Bd else Bu);
                  Code := Choose (4);
               when others =>
                  Kd_K := Motion;
                  Code := 0;
            end case;
            D := Dest (St, Nm);
            Stamp (St, D, G, E);
            Ev := (K => Kd_K, A => D, Code => Code, Gen => G, Ep => E);
         end;
      elsif K < 55 then
         Ev := (K => (if Choose (2) = 0 then Chord else Menu), A => Hub, Code => 0, Gen => 0, Ep => 0);
      elsif K < 70 then
         declare
            T : U8;
         begin
            if Choose (32) = 0 then
               T := 200;
            else
               T := U8 (Choose (Nm + 1));
            end if;
            Ev := (K => Req, A => T, Code => 0, Gen => 0, Ep => 0);
         end;
      elsif K < 80 then
         declare
            M : U8;
            E : U32;
            V : Kind;
         begin
            if St.Waiting /= None and then Choose (10) < 7 then
               M := St.Waiting;
            else
               M := U8 (Choose (Nm) + 1);
            end if;
            if Choose (10) < 8 then
               E := St.Epoch;
            else
               E := Choose (Sat_Add2 (St.Epoch));
            end if;
            V := (if Choose (2) = 0 then Vok else Vto);
            Ev := (K => V, A => M, Code => 0, Gen => 0, Ep => E);
         end;
      else
         declare
            M : constant U8 := U8 (Choose (Nm) + 1);
         begin
            Ev := (K => (if K < 86 then Rst elsif K < 93 then Dis else Rec), A => M, Code => 0, Gen => 0, Ep => 0);
         end;
      end if;
      return Ev;
   end Gen_Event;

   Seed  : Unsigned_64 := 1;
   Cases : Unsigned_64 := 1_000_000;
   Steps : Unsigned_64 := 0;
   Out_B : Out_Buf;
   T0    : constant Ada.Calendar.Time := Ada.Calendar.Clock;

begin
   if Ada.Command_Line.Argument_Count >= 1 then
      Seed := Unsigned_64'Value (Ada.Command_Line.Argument (1));
   end if;
   if Ada.Command_Line.Argument_Count >= 2 then
      Cases := Unsigned_64'Value (Ada.Command_Line.Argument (2));
   end if;
   Rng_State := Seed;
   for Case_No in 0 .. Cases - 1 loop
      declare
         Nm    : constant U32 := 3 + Choose (18);
         Len   : constant U32 := 8 + Choose (25);
         St    : State;
         G     : Focus_Check.Ghost;
      begin
         Focus_Check.Reset (G);
         for Step_No in 0 .. Len - 1 loop
            declare
               Ev  : constant Event := Gen_Event (St, Nm);
               Pre : constant State := St;
               Bad : Natural;
            begin
               Step (St, Ev, Out_B);
               Steps := Steps + 1;
               Bad := Focus_Check.After_Step (Pre, Ev, Out_B, St, G);
               if Bad /= 0 then
                  Put_Line ("VIOLATION invariant" & Natural'Image (Bad) & " at case" & Unsigned_64'Image (Case_No)
                            & " step" & U32'Image (Step_No) & " (seed" & Unsigned_64'Image (Seed) & ")");
                  Ada.Command_Line.Set_Exit_Status (1);
                  return;
               end if;
               Feed (U32 (Out_B.N));
               for I in 0 .. Out_B.N - 1 loop
                  Feed (U32 (Act_Kind'Pos (Out_B.Acts (I).T)));
                  Feed (U32 (Out_B.Acts (I).Dest));
                  Feed (U32 (Kind'Pos (Out_B.Acts (I).K)));
                  Feed (Out_B.Acts (I).Code);
                  Feed (Out_B.Acts (I).Gen);
                  Feed (Out_B.Acts (I).Ep);
               end loop;
               Feed (U32 (St.Focus));
               Feed (St.Epoch);
               Feed (U32 (St.Waiting));
               Feed (U32 (St.Nkeys));
               Feed (U32 (St.Nbtns));
               declare
                  Gsum  : U32 := 0;
                  Cbits : U32 := 0;
               begin
                  for M in 1 .. Max_M loop
                     Gsum := Gsum + St.Gen (M);
                     if St.Conn (M) then
                        Cbits := Cbits + 1;
                     end if;
                  end loop;
                  Feed (Gsum);
                  Feed (Cbits);
               end;
            end;
         end loop;
      end;
   end loop;
   declare
      Secs : constant Duration := Ada.Calendar."-" (Ada.Calendar.Clock, T0);
      Hex  : constant String := "0123456789abcdef";
      H    : Unsigned_64 := Hash;
      Txt  : String (1 .. 16);
   begin
      for I in reverse 1 .. 16 loop
         Txt (I) := Hex (Integer (H and 15) + 1);
         H := Shift_Right (H, 4);
      end loop;
      Put_Line ("ada fuzz: seed=" & Unsigned_64'Image (Seed) & " cases=" & Unsigned_64'Image (Cases) & " steps="
                & Unsigned_64'Image (Steps) & " violations=0 hash=" & Txt & " seconds=" & Duration'Image (Secs));
   end;
end Fuzz_Run;
