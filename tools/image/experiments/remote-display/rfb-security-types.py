"""Print the security types a VNC server offers. usage: rfb-security-types.py PORT"""
import socket,sys
s=socket.create_connection(("127.0.0.1",int(sys.argv[1]))); s.recv(12); s.sendall(b"RFB 003.008\n")
n=s.recv(1)[0]; print("port",sys.argv[1],"security types offered:",list(s.recv(n)))
