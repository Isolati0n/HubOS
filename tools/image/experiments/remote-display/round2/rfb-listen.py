"""Connect to a VNC server (security None), request updates, print every ServerCutText (type 3) that arrives within N seconds. usage: rfb-listen.py PORT SECONDS"""
import socket,struct,sys,time,select
s=socket.create_connection(("127.0.0.1",int(sys.argv[1])))
s.recv(12); s.sendall(b"RFB 003.008\n")
n=s.recv(1)[0]; s.recv(n); s.sendall(bytes([1])); s.recv(4); s.sendall(b"\x01")
time.sleep(1); s.settimeout(0.3)
buf=b""; t0=time.time()
print("listening", flush=True)
# ignore framebuffer data: scan for type-3 messages heuristically is unreliable; so request no encodings except raw 1x1 updates
s.sendall(struct.pack(">BxH",2,1)+struct.pack(">i",0))   # SetEncodings: raw only
s.sendall(struct.pack(">BBHHHH",3,0,0,0,1,1))
while time.time()-t0<float(sys.argv[2]):
    try: d=s.recv(1<<20)
    except Exception: continue
    buf+=d
    while buf:
        t=buf[0]
        if t==3:
            if len(buf)<8: break
            ln=struct.unpack(">I",buf[4:8])[0]
            if len(buf)<8+ln: break
            print("ServerCutText", repr(buf[8:8+ln]), flush=True); buf=buf[8+ln:]
        elif t==0:
            if len(buf)<4: break
            nr=struct.unpack(">H",buf[2:4])[0]; p=4; ok=True
            for _ in range(nr):
                if len(buf)<p+12: ok=False;break
                x,y,w,h,e=struct.unpack(">HHHHi",buf[p:p+12]); p+=12
                if e==0: 
                    sz=w*h*4
                    if len(buf)<p+sz: ok=False;break
                    p+=sz
                else: print("pseudo-encoding rect", e, flush=True)
            if not ok: break
            buf=buf[p:]
            s.sendall(struct.pack(">BBHHHH",3,1,0,0,1,1))
        elif t==2: buf=buf[1:]
        else: print("other server msg", t, flush=True); buf=b""
