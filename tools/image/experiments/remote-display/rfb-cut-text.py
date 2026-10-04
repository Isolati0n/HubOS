"""Send one RFB ClientCutText to a VNC server and stay connected 8 s. usage: rfb-cut-text.py PORT TEXT"""
import socket,struct,sys,time
s=socket.create_connection(("127.0.0.1",int(sys.argv[1])))
s.recv(12); s.sendall(b"RFB 003.008\n")
n=s.recv(1)[0]; s.recv(n); s.sendall(bytes([1])); s.recv(4); s.sendall(b"\x01"); time.sleep(1)
t=sys.argv[2].encode()
s.sendall(struct.pack(">BxxxI",6,len(t))+t); time.sleep(1)
s.settimeout(0.5)
try:
    while s.recv(1<<20): pass
except Exception: pass
