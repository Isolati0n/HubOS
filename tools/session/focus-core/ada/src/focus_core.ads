--  The focus core of the session layer (docs/proposals/focus-core.md): step (state, event) -> (new state, actions).
--  Ada/SPARK candidate. Pure: no I/O, no access types, fixed-size arrays only.
--  Machines are 1 .. Max_M, the hub is 0, "nobody" is 255. Same rules as docs/models/focus-core/FocusCore.tla.
with Interfaces;

package Focus_Core with SPARK_Mode => On is

   Max_M    : constant := 20;
   Max_Keys : constant := 32;   --  placeholder limits (owner question 1)
   Max_Btns : constant := 8;
   Max_Acts : constant := 64;

   subtype U8 is Interfaces.Unsigned_8;
   subtype U32 is Interfaces.Unsigned_32;
   use type U8;
   use type U32;

   Hub  : constant U8 := 0;
   None : constant U8 := 255;

   type Kind is (Kd, Ku, Bd, Bu, Motion, Chord, Menu, Req, Vok, Vto, Rst, Dis, Rec);

   type Event is record
      K    : Kind := Motion;
      A    : U8 := 0;
      Code : U32 := 0;
      Gen  : U32 := 0;
      Ep   : U32 := 0;
   end record;

   type Act_Kind is (Deliver, Deliver_Hub, Drop, Flash, Synth_Ku, Synth_Bu, Release_All);

   type Act is record
      T    : Act_Kind := Drop;
      Dest : U8 := 0;
      K    : Kind := Motion;
      Code : U32 := 0;
      Gen  : U32 := 0;
      Ep   : U32 := 0;
   end record;

   type Act_Array is array (0 .. Max_Acts - 1) of Act;

   type Out_Buf is record
      Acts : Act_Array := (others => (others => <>));
      N    : Natural range 0 .. Max_Acts := 0;
   end record;

   type Held is record
      Code : U32 := 0;
      Dest : U8 := 0;
   end record;

   type Key_List is array (0 .. Max_Keys - 1) of Held;
   type Btn_List is array (0 .. Max_Btns - 1) of Held;
   type U32_Array is array (0 .. Max_M) of U32;
   type Bool_Array is array (0 .. Max_M) of Boolean;

   type State is record
      Focus   : U8 := Hub;
      Epoch   : U32 := 0;
      Gen     : U32_Array := (others => 0);
      Conn    : Bool_Array := (others => True);
      Waiting : U8 := None;
      Owed    : Bool_Array := (others => False);
      Keys    : Key_List := (others => (others => <>));
      Nkeys   : Natural range 0 .. Max_Keys := 0;
      Btns    : Btn_List := (others => (others => <>));
      Nbtns   : Natural range 0 .. Max_Btns := 0;
   end record;

   function Is_Machine (M : U8) return Boolean is (M >= 1 and then M <= Max_M);

   function Count_Keys (S : State; Dest : U8) return Natural
     with Post => Count_Keys'Result <= S.Nkeys;
   function Count_Btns (S : State; Dest : U8) return Natural
     with Post => Count_Btns'Result <= S.Nbtns;

   --  The state is valid when the held lists are within their limits, a waiting machine means the hub has the
   --  focus, only the focus machine holds anything, and the hub itself never holds a key or button.
   function Valid (S : State) return Boolean is
     (S.Nkeys <= Max_Keys
      and then S.Nbtns <= Max_Btns
      and then (if S.Waiting /= None then S.Focus = Hub)
      and then (if S.Focus = Hub then S.Nkeys = 0 and then S.Nbtns = 0)
      and then (for all I in 0 .. S.Nkeys - 1 => S.Keys (I).Dest = S.Focus)
      and then (for all I in 0 .. S.Nbtns - 1 => S.Btns (I).Dest = S.Focus));

   procedure Step (S : in out State; Ev : Event; Output : in out Out_Buf)
     with Pre  => Valid (S),
          Post => Valid (S)
                  and then S.Epoch >= S.Epoch'Old
                  and then (if Ev.K in Chord | Menu then Output.N = 1 and then Output.Acts (0).T = Deliver_Hub)
                  and then Output.N <= Max_Acts;

end Focus_Core;
