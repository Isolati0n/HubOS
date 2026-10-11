--  Runs shared trace files through the Ada core and compares every step (same format as the other candidates).
--  Usage: trace_run FILE...
with Ada.Text_IO; use Ada.Text_IO;
with Ada.Command_Line;
with Ada.Strings.Unbounded; use Ada.Strings.Unbounded;
with Ada.Strings.Fixed;
with Focus_Core; use Focus_Core;
with Interfaces; use Interfaces;

procedure Trace_Run is

   Names : constant array (Kind) of access constant String :=
     (Kd => new String'("kd"), Ku => new String'("ku"), Bd => new String'("bd"), Bu => new String'("bu"),
      Motion => new String'("motion"), Chord => new String'("chord"), Menu => new String'("menu"),
      Req => new String'("req"), Vok => new String'("vok"), Vto => new String'("vto"), Rst => new String'("rst"),
      Dis => new String'("dis"), Rec => new String'("rec"));

   function Img (N : Long_Long_Integer) return String is
      S : constant String := Long_Long_Integer'Image (N);
   begin
      return S (S'First + 1 .. S'Last);
   end Img;

   function Act_Line (A : Act) return String is
   begin
      case A.T is
         when Deliver     => return "D " & Img (Long_Long_Integer (A.Dest)) & " " & Names (A.K).all & " " & Img (Long_Long_Integer (A.Code));
         when Deliver_Hub => return "H " & Names (A.K).all & " " & Img (Long_Long_Integer (A.Code));
         when Drop        => return "X";
         when Flash       => return "F " & Img (Long_Long_Integer (A.Dest));
         when Synth_Ku    => return "SK " & Img (Long_Long_Integer (A.Dest)) & " " & Img (Long_Long_Integer (A.Code));
         when Synth_Bu    => return "SB " & Img (Long_Long_Integer (A.Dest)) & " " & Img (Long_Long_Integer (A.Code));
         when Release_All => return "RA " & Img (Long_Long_Integer (A.Dest));
      end case;
   end Act_Line;

   --  Held list printed sorted by (dest, code); "-" if empty.
   function Held_List_Keys (S : State) return String is
      R : Unbounded_String;
      Used : array (0 .. Max_Keys - 1) of Boolean := (others => False);
   begin
      for Round in 1 .. S.Nkeys loop
         declare
            Best : Integer := -1;
         begin
            for I in 0 .. S.Nkeys - 1 loop
               if not Used (I) and then
                 (Best < 0 or else S.Keys (I).Dest < S.Keys (Best).Dest
                  or else (S.Keys (I).Dest = S.Keys (Best).Dest and then S.Keys (I).Code < S.Keys (Best).Code))
               then
                  Best := I;
               end if;
            end loop;
            Used (Best) := True;
            if Length (R) > 0 then
               Append (R, ",");
            end if;
            Append (R, Img (Long_Long_Integer (S.Keys (Best).Dest)) & ":" & Img (Long_Long_Integer (S.Keys (Best).Code)));
         end;
      end loop;
      return (if Length (R) = 0 then "-" else To_String (R));
   end Held_List_Keys;

   function Held_List_Btns (S : State) return String is
      R : Unbounded_String;
      Used : array (0 .. Max_Btns - 1) of Boolean := (others => False);
   begin
      for Round in 1 .. S.Nbtns loop
         declare
            Best : Integer := -1;
         begin
            for I in 0 .. S.Nbtns - 1 loop
               if not Used (I) and then
                 (Best < 0 or else S.Btns (I).Dest < S.Btns (Best).Dest
                  or else (S.Btns (I).Dest = S.Btns (Best).Dest and then S.Btns (I).Code < S.Btns (Best).Code))
               then
                  Best := I;
               end if;
            end loop;
            Used (Best) := True;
            if Length (R) > 0 then
               Append (R, ",");
            end if;
            Append (R, Img (Long_Long_Integer (S.Btns (Best).Dest)) & ":" & Img (Long_Long_Integer (S.Btns (Best).Code)));
         end;
      end loop;
      return (if Length (R) = 0 then "-" else To_String (R));
   end Held_List_Btns;

   function State_Line (S : State; N : Natural) return String is
      G, C, O : Unbounded_String;
   begin
      for M in 1 .. N loop
         if M > 1 then
            Append (G, ","); Append (C, ","); Append (O, ",");
         end if;
         Append (G, Img (Long_Long_Integer (S.Gen (M))));
         Append (C, (if S.Conn (M) then "1" else "0"));
         Append (O, (if S.Owed (M) then "1" else "0"));
      end loop;
      return "state focus=" & Img (Long_Long_Integer (S.Focus)) & " epoch=" & Img (Long_Long_Integer (S.Epoch))
        & " waiting=" & (if S.Waiting = None then "-" else Img (Long_Long_Integer (S.Waiting)))
        & " gen=" & To_String (G) & " conn=" & To_String (C) & " owed=" & To_String (O)
        & " keys=" & Held_List_Keys (S) & " btns=" & Held_List_Btns (S);
   end State_Line;

   --  split a line on blanks
   type Words is array (1 .. 8) of Unbounded_String;
   procedure Split (L : String; W : out Words; Count : out Natural) is
      I : Natural := L'First;
   begin
      Count := 0;
      W := (others => Null_Unbounded_String);
      while I <= L'Last loop
         while I <= L'Last and then L (I) = ' ' loop
            I := I + 1;
         end loop;
         exit when I > L'Last;
         declare
            J : Natural := I;
         begin
            while J <= L'Last and then L (J) /= ' ' loop
               J := J + 1;
            end loop;
            Count := Count + 1;
            if Count <= W'Last then
               W (Count) := To_Unbounded_String (L (I .. J - 1));
            end if;
            I := J;
         end;
      end loop;
   end Split;

   --  tiny insertion sort of the action lines
   type Line_Array is array (1 .. Max_Acts) of Unbounded_String;

   Steps, Scenarios, Bad : Natural := 0;

begin
   for Arg in 1 .. Ada.Command_Line.Argument_Count loop
      declare
         F       : File_Type;
         N       : Natural := 3;
         St      : State;
         Output  : Out_Buf;
         Name    : Unbounded_String;
         Pending : Boolean := False;
         Ev      : Event;
         Want    : Line_Array;
         Nwant   : Natural := 0;
         Line_No : Natural := 0;
      begin
         Open (F, In_File, Ada.Command_Line.Argument (Arg));
         while not End_Of_File (F) loop
            declare
               Line : constant String := Ada.Strings.Fixed.Trim (Get_Line (F), Ada.Strings.Both);
            begin
               Line_No := Line_No + 1;
               if Line'Length = 0 or else Line (Line'First) = '#' then
                  null;
               elsif Line'Length > 2 and then Line (Line'First .. Line'First + 1) = "n " then
                  N := Natural'Value (Line (Line'First + 2 .. Line'Last));
               elsif Line'Length > 9 and then Line (Line'First .. Line'First + 8) = "scenario " then
                  Name := To_Unbounded_String (Line (Line'First + 9 .. Line'Last));
                  St := (others => <>);
                  Scenarios := Scenarios + 1;
               elsif Line = "end" then
                  null;
               elsif Line'Length > 3 and then Line (Line'First .. Line'First + 2) = "ev " then
                  declare
                     W : Words;
                     C : Natural;
                  begin
                     Split (Line, W, C);
                     Pending := False;
                     for K in Kind loop
                        if To_String (W (2)) = Names (K).all then
                           Ev := (K => K, A => U8'Value (To_String (W (3))), Code => U32'Value (To_String (W (4))),
                                  Gen => U32'Value (To_String (W (5))), Ep => U32'Value (To_String (W (6))));
                           Pending := True;
                        end if;
                     end loop;
                     if not Pending then
                        Put_Line (Standard_Error, "bad event line " & Natural'Image (Line_No));
                        Bad := Bad + 1;
                     end if;
                     Nwant := 0;
                  end;
               elsif Line'Length > 4 and then Line (Line'First .. Line'First + 3) = "act " then
                  Nwant := Nwant + 1;
                  Want (Nwant) := To_Unbounded_String (Line (Line'First + 4 .. Line'Last));
               elsif Line'Length > 6 and then Line (Line'First .. Line'First + 5) = "state " then
                  if Pending then
                     Pending := False;
                     Step (St, Ev, Output);
                     Steps := Steps + 1;
                     declare
                        Got : Line_Array;
                     begin
                        for I in 0 .. Output.N - 1 loop
                           Got (I + 1) := To_Unbounded_String (Act_Line (Output.Acts (I)));
                        end loop;
                        --  sort both (insertion sort)
                        for A in 2 .. Output.N loop
                           declare
                              Key : constant Unbounded_String := Got (A);
                              J   : Integer := A - 1;
                           begin
                              while J >= 1 and then Got (J) > Key loop
                                 Got (J + 1) := Got (J);
                                 J := J - 1;
                              end loop;
                              Got (J + 1) := Key;
                           end;
                        end loop;
                        for A in 2 .. Nwant loop
                           declare
                              Key : constant Unbounded_String := Want (A);
                              J   : Integer := A - 1;
                           begin
                              while J >= 1 and then Want (J) > Key loop
                                 Want (J + 1) := Want (J);
                                 J := J - 1;
                              end loop;
                              Want (J + 1) := Key;
                           end;
                        end loop;
                        declare
                           Same : Boolean := Output.N = Nwant;
                        begin
                           if Same then
                              for I in 1 .. Nwant loop
                                 if Got (I) /= Want (I) then
                                    Same := False;
                                 end if;
                              end loop;
                           end if;
                           if not Same then
                              Put_Line (Standard_Error, Ada.Command_Line.Argument (Arg) & ":" & Natural'Image (Line_No) & " [" & To_String (Name) & "] actions differ");
                              Bad := Bad + 1;
                           end if;
                        end;
                     end;
                     if State_Line (St, N) /= Line then
                        Put_Line (Standard_Error, Ada.Command_Line.Argument (Arg) & ":" & Natural'Image (Line_No) & " [" & To_String (Name) & "] state differs");
                        Put_Line (Standard_Error, "  want " & Line);
                        Put_Line (Standard_Error, "  got  " & State_Line (St, N));
                        Bad := Bad + 1;
                     end if;
                  end if;
               end if;
            end;
         end loop;
         Close (F);
      end;
   end loop;
   Put_Line ("ada trace: scenarios=" & Natural'Image (Scenarios) & " steps=" & Natural'Image (Steps) & " mismatches=" & Natural'Image (Bad));
   if Bad > 0 then
      Ada.Command_Line.Set_Exit_Status (1);
   end if;
end Trace_Run;
