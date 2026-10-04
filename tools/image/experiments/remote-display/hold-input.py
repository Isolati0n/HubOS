"""Hold a VNC connection open so wayvnc keeps its virtual keyboard and pointer (gives a headless sway a keyboard seat). usage: hold-input.py PORT"""
import socket,struct,sys,time
s=socket.create_connection(("127.0.0.1",int(sys.argv[1])))
s.recv(12); s.sendall(b"RFB 003.008\n")
n=s.recv(1)[0]; types=s.recv(n); s.sendall(bytes([1]))
s.recv(4); s.sendall(b"\x01")
s.recv(24+4096) if False else None
time.sleep(1)
s.settimeout(0.2)
# FramebufferUpdateRequest, then keep alive and send a key
s.sendall(struct.pack(">BBHHHH",3,0,0,0,100,100))
s.sendall(struct.pack(">BBxxI",4,1,0xffe1)); s.sendall(struct.pack(">BBxxI",4,0,0xffe1))
while True:
    try: s.recv(1<<20)
    except Exception: pass
    time.sleep(0.5)
