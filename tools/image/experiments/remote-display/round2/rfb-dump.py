"""Byte-recording relay (EXPERIMENT): forwards a TCP connection on loopback and writes every byte (both directions) to a file, so a test can grep it for a clipboard text and show whether the link is encrypted. usage: rfb-dump.py LISTEN_PORT SERVER_PORT OUTFILE"""
import socket,sys,threading
lp,sp,out=int(sys.argv[1]),int(sys.argv[2]),sys.argv[3]
f=open(out,"wb")
def pump(a,b):
    try:
        while True:
            d=a.recv(65536)
            if not d: break
            f.write(d); f.flush(); b.sendall(d)
    except OSError: pass
s=socket.socket(); s.setsockopt(socket.SOL_SOCKET,socket.SO_REUSEADDR,1); s.bind(("127.0.0.1",lp)); s.listen(2)
while True:
    c,_=s.accept(); u=socket.create_connection(("127.0.0.1",sp))
    threading.Thread(target=pump,args=(c,u),daemon=True).start(); threading.Thread(target=pump,args=(u,c),daemon=True).start()
