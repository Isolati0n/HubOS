package body Focus_Core with SPARK_Mode => On is

   function Count_Keys (S : State; Dest : U8) return Natural is
      C : Natural := 0;
   begin
      for I in 0 .. S.Nkeys - 1 loop
         pragma Loop_Invariant (C <= I);
         if S.Keys (I).Dest = Dest then
            C := C + 1;
         end if;
      end loop;
      return C;
   end Count_Keys;

   function Count_Btns (S : State; Dest : U8) return Natural is
      C : Natural := 0;
   begin
      for I in 0 .. S.Nbtns - 1 loop
         pragma Loop_Invariant (C <= I);
         if S.Btns (I).Dest = Dest then
            C := C + 1;
         end if;
      end loop;
      return C;
   end Count_Btns;

   procedure Push (O : in out Out_Buf; A : Act) with Inline is
   begin
      if O.N < Max_Acts then
         O.Acts (O.N) := A;
         O.N := O.N + 1;
      end if;
   end Push;

   function Sat_Inc (X : U32) return U32 is (if X = U32'Last then X else X + 1);

   procedure Remove_Dest_Keys (S : in out State; Dest : U8) is
      W : Natural := 0;
   begin
      for R in 0 .. S.Nkeys - 1 loop
         pragma Loop_Invariant (W <= R);
         if S.Keys (R).Dest /= Dest then
            S.Keys (W) := S.Keys (R);
            W := W + 1;
         end if;
      end loop;
      S.Nkeys := W;
   end Remove_Dest_Keys;

   procedure Remove_Dest_Btns (S : in out State; Dest : U8) is
      W : Natural := 0;
   begin
      for R in 0 .. S.Nbtns - 1 loop
         pragma Loop_Invariant (W <= R);
         if S.Btns (R).Dest /= Dest then
            S.Btns (W) := S.Btns (R);
            W := W + 1;
         end if;
      end loop;
      S.Nbtns := W;
   end Remove_Dest_Btns;

   function Find_Key (S : State; Code : U32; Dest : U8) return Integer is
   begin
      for I in 0 .. S.Nkeys - 1 loop
         if S.Keys (I).Code = Code and then S.Keys (I).Dest = Dest then
            return I;
         end if;
      end loop;
      return -1;
   end Find_Key;

   function Find_Btn (S : State; Code : U32; Dest : U8) return Integer is
   begin
      for I in 0 .. S.Nbtns - 1 loop
         if S.Btns (I).Code = Code and then S.Btns (I).Dest = Dest then
            return I;
         end if;
      end loop;
      return -1;
   end Find_Btn;

   procedure Remove_Key_At (S : in out State; I : Natural) with Pre => I < S.Nkeys is
   begin
      for J in I .. S.Nkeys - 2 loop
         S.Keys (J) := S.Keys (J + 1);
      end loop;
      S.Nkeys := S.Nkeys - 1;
   end Remove_Key_At;

   procedure Remove_Btn_At (S : in out State; I : Natural) with Pre => I < S.Nbtns is
   begin
      for J in I .. S.Nbtns - 2 loop
         S.Btns (J) := S.Btns (J + 1);
      end loop;
      S.Nbtns := S.Nbtns - 1;
   end Remove_Btn_At;

   procedure Input (S : in out State; Ev : Event; Output : in out Out_Buf) is
      D  : constant U8 := Ev.A;
      Ok : constant Boolean :=
        Is_Machine (D) and then D = S.Focus and then Ev.Gen = S.Gen (Integer (D))
        and then Ev.Ep = S.Epoch and then S.Conn (Integer (D));
   begin
      if Ok then
         case Ev.K is
            when Kd =>
               if Find_Key (S, Ev.Code, D) < 0 then
                  if S.Nkeys >= Max_Keys then
                     Push (Output, (T => Drop, Dest => D, K => Ev.K, Code => Ev.Code, Gen => 0, Ep => 0));
                     return;
                  end if;
                  S.Keys (S.Nkeys) := (Code => Ev.Code, Dest => D);
                  S.Nkeys := S.Nkeys + 1;
               end if;
            when Ku =>
               declare
                  I : constant Integer := Find_Key (S, Ev.Code, D);
               begin
                  if I >= 0 and then I < S.Nkeys then
                     Remove_Key_At (S, I);
                  end if;
               end;
            when Bd =>
               if Find_Btn (S, Ev.Code, D) < 0 then
                  if S.Nbtns >= Max_Btns then
                     Push (Output, (T => Drop, Dest => D, K => Ev.K, Code => Ev.Code, Gen => 0, Ep => 0));
                     return;
                  end if;
                  S.Btns (S.Nbtns) := (Code => Ev.Code, Dest => D);
                  S.Nbtns := S.Nbtns + 1;
               end if;
            when Bu =>
               declare
                  I : constant Integer := Find_Btn (S, Ev.Code, D);
               begin
                  if I >= 0 and then I < S.Nbtns then
                     Remove_Btn_At (S, I);
                  end if;
               end;
            when others =>
               null;
         end case;
         Push (Output, (T => Deliver, Dest => D, K => Ev.K, Code => Ev.Code, Gen => Ev.Gen, Ep => Ev.Ep));
      elsif D = Hub and then S.Focus = Hub and then Ev.Ep = S.Epoch then
         Push (Output, (T => Deliver_Hub, Dest => Hub, K => Ev.K, Code => Ev.Code, Gen => 0, Ep => Ev.Ep));
      else
         Push (Output, (T => Drop, Dest => D, K => Ev.K, Code => Ev.Code, Gen => 0, Ep => 0));
         if Is_Machine (D) and then not S.Conn (Integer (D)) then
            Push (Output, (T => Flash, Dest => D, K => Ev.K, Code => 0, Gen => 0, Ep => 0));
         end if;
      end if;
   end Input;

   procedure Request (S : in out State; T : U8; Output : in out Out_Buf) is
      Old : constant U8 := S.Focus;
   begin
      if T /= Hub and then not Is_Machine (T) then
         return;
      end if;
      S.Epoch := Sat_Inc (S.Epoch);
      if Is_Machine (Old) then
         if S.Conn (Integer (Old)) then
            for I in 0 .. S.Nkeys - 1 loop
               if S.Keys (I).Dest = Old then
                  Push (Output, (T => Synth_Ku, Dest => Old, K => Ku, Code => S.Keys (I).Code, Gen => 0, Ep => 0));
               end if;
            end loop;
            for I in 0 .. S.Nbtns - 1 loop
               if S.Btns (I).Dest = Old then
                  Push (Output, (T => Synth_Bu, Dest => Old, K => Bu, Code => S.Btns (I).Code, Gen => 0, Ep => 0));
               end if;
            end loop;
         elsif Count_Keys (S, Old) + Count_Btns (S, Old) > 0 then
            S.Owed (Integer (Old)) := True;
         end if;
      end if;
      S.Nkeys := 0;
      S.Nbtns := 0;
      S.Focus := Hub;
      S.Waiting := (if T = Hub then None else T);
   end Request;

   procedure Restart (S : in out State; M : U8) is
   begin
      if not Is_Machine (M) then
         return;
      end if;
      declare
         Mi          : constant Integer := Integer (M);
         Was_Focus   : constant Boolean := S.Focus = M;
         Was_Waiting : constant Boolean := S.Waiting = M;
      begin
         S.Gen (Mi) := Sat_Inc (S.Gen (Mi));
         S.Conn (Mi) := False;
         Remove_Dest_Keys (S, M);
         Remove_Dest_Btns (S, M);
         S.Owed (Mi) := True;
         if Was_Focus or else Was_Waiting then
            S.Epoch := Sat_Inc (S.Epoch);
            if Was_Focus then
               S.Focus := Hub;
            end if;
            S.Waiting := M;
         end if;
      end;
   end Restart;

   procedure Step (S : in out State; Ev : Event; Output : in out Out_Buf) is
   begin
      Output.N := 0;
      case Ev.K is
         when Kd | Ku | Bd | Bu | Motion =>
            Input (S, Ev, Output);
         when Chord | Menu =>
            Push (Output, (T => Deliver_Hub, Dest => Hub, K => Ev.K, Code => Ev.Code, Gen => 0, Ep => 0));
         when Req =>
            Request (S, Ev.A, Output);
         when Vok =>
            if Is_Machine (Ev.A) and then S.Waiting = Ev.A and then Ev.Ep = S.Epoch and then S.Conn (Integer (Ev.A)) then
               S.Focus := Ev.A;
               S.Waiting := None;
            end if;
         when Vto =>
            if Is_Machine (Ev.A) and then S.Waiting = Ev.A and then Ev.Ep = S.Epoch then
               S.Focus := Hub;
               S.Waiting := None;
            end if;
         when Rst =>
            Restart (S, Ev.A);
         when Dis =>
            if Is_Machine (Ev.A) and then S.Conn (Integer (Ev.A)) then
               S.Conn (Integer (Ev.A)) := False;
               if S.Waiting = Ev.A then
                  S.Waiting := None;
               end if;
            end if;
         when Rec =>
            if Is_Machine (Ev.A) and then not S.Conn (Integer (Ev.A)) then
               S.Conn (Integer (Ev.A)) := True;
               Remove_Dest_Keys (S, Ev.A);
               Remove_Dest_Btns (S, Ev.A);
               S.Owed (Integer (Ev.A)) := False;
               Push (Output, (T => Release_All, Dest => Ev.A, K => Motion, Code => 0, Gen => 0, Ep => 0));
            end if;
      end case;
   end Step;

end Focus_Core;
