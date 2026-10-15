with Interfaces; use Interfaces;

package body Focus_Check is

   procedure Reset (G : out Ghost) is
   begin
      G := (Keys => (others => 0), Btns => (others => 0));
   end Reset;

   procedure Apply (G : in out Ghost; O : Out_Buf) is
   begin
      for I in 0 .. O.N - 1 loop
         declare
            A  : constant Act := O.Acts (I);
            D  : constant Integer := Integer (A.Dest);
            Kb : constant Natural := Natural (A.Code mod 64);
            Bb : constant Natural := Natural (A.Code mod 8);
         begin
            case A.T is
               when Deliver =>
                  case A.K is
                     when Kd => G.Keys (D) := G.Keys (D) or Shift_Left (Unsigned_64'(1), Kb);
                     when Ku => G.Keys (D) := G.Keys (D) and not Shift_Left (Unsigned_64'(1), Kb);
                     when Bd => G.Btns (D) := G.Btns (D) or Shift_Left (U8'(1), Bb);
                     when Bu => G.Btns (D) := G.Btns (D) and not Shift_Left (U8'(1), Bb);
                     when others => null;
                  end case;
               when Synth_Ku => G.Keys (D) := G.Keys (D) and not Shift_Left (Unsigned_64'(1), Kb);
               when Synth_Bu => G.Btns (D) := G.Btns (D) and not Shift_Left (U8'(1), Bb);
               when Release_All =>
                  G.Keys (D) := 0;
                  G.Btns (D) := 0;
               when others => null;
            end case;
         end;
      end loop;
   end Apply;

   function After_Step (Pre : State; Ev : Event; Output : Out_Buf; Post : State; G : in out Ghost) return Natural is
      Delivers : Natural := 0;
   begin
      if Output.N > Max_Acts or else Post.Nkeys > Max_Keys or else Post.Nbtns > Max_Btns then
         return 7;
      end if;
      for I in 0 .. Output.N - 1 loop
         declare
            A : constant Act := Output.Acts (I);
         begin
            case A.T is
               when Deliver =>
                  Delivers := Delivers + 1;
                  if not (Is_Machine (A.Dest) and then A.Dest = Pre.Focus and then A.Gen = Pre.Gen (Integer (A.Dest))
                          and then A.Ep = Pre.Epoch and then Pre.Conn (Integer (A.Dest)))
                  then
                     return 1;
                  end if;
               when Deliver_Hub => Delivers := Delivers + 1;
               when others => null;
            end case;
         end;
      end loop;
      if Delivers > 1 then
         return 2;
      end if;
      for I in 0 .. Output.N - 1 loop
         if Output.Acts (I).T = Deliver_Hub and then Output.Acts (I).K not in Chord | Menu
           and then not (Pre.Focus = Hub and then Output.Acts (I).Ep = Pre.Epoch)
         then
            return 1;
         end if;
      end loop;
      if Post.Epoch < Pre.Epoch then
         return 4;
      end if;
      if Ev.K = Req and then (Ev.A = Hub or else Is_Machine (Ev.A)) and then Pre.Epoch /= U32'Last
        and then Post.Epoch /= Pre.Epoch + 1
      then
         return 4;
      end if;
      if Ev.K in Chord | Menu then
         declare
            Found : Boolean := False;
         begin
            for I in 0 .. Output.N - 1 loop
               if Output.Acts (I).T = Deliver_Hub and then Output.Acts (I).K = Ev.K then
                  Found := True;
               end if;
            end loop;
            if not Found then
               return 5;
            end if;
         end;
      end if;
      declare
         Is_V    : constant Boolean := Ev.K in Vok | Vto;
         Valid_V : constant Boolean :=
           Is_V and then Is_Machine (Ev.A) and then Pre.Waiting = Ev.A and then Ev.Ep = Pre.Epoch
           and then (Ev.K = Vto or else Pre.Conn (Integer (Ev.A)));
      begin
         if Is_V and then not Valid_V and then (Post.Focus /= Pre.Focus or else Post.Waiting /= Pre.Waiting) then
            return 6;
         end if;
         if Ev.K = Vto and then Valid_V and then not (Post.Waiting = None and then Post.Focus = Hub) then
            return 6;
         end if;
         if Post.Waiting /= None and then Post.Focus /= Hub then
            return 6;
         end if;
      end;
      Apply (G, Output);
      for M in 1 .. Max_M loop
         declare
            Mk : constant Natural := Count_Keys (Post, U8 (M));
            Mb : constant Natural := Count_Btns (Post, U8 (M));
         begin
            if U8 (M) /= Post.Focus and then (Mk > 0 or else Mb > 0) then
               return 3;
            end if;
            if not Post.Owed (M) then
               declare
                  Bk : Unsigned_64 := 0;
                  Bb : U8 := 0;
               begin
                  for J in 0 .. Post.Nkeys - 1 loop
                     if Post.Keys (J).Dest = U8 (M) then
                        Bk := Bk or Shift_Left (Unsigned_64'(1), Natural (Post.Keys (J).Code mod 64));
                     end if;
                  end loop;
                  for J in 0 .. Post.Nbtns - 1 loop
                     if Post.Btns (J).Dest = U8 (M) then
                        Bb := Bb or Shift_Left (U8'(1), Natural (Post.Btns (J).Code mod 8));
                     end if;
                  end loop;
                  if G.Keys (M) /= Bk or else G.Btns (M) /= Bb then
                     return 3;
                  end if;
               end;
            end if;
         end;
      end loop;
      return 0;
   end After_Step;

end Focus_Check;
