#!/usr/bin/env python3
"""findnode.py node-prop KEY VALUE   : ids of nodes whose property KEY equals VALUE
   findnode.py pid PID              : ids of the nodes made by the client process PID (client property application.process.id)
Uses pw-dump. The process id is a property of the client object, not of the node, so it is joined by client.id."""
import json, subprocess, sys
dump = json.loads(subprocess.check_output(["pw-dump"]))
if sys.argv[1] == "pid":
    clients = {o["id"] for o in dump if o.get("type") == "PipeWire:Interface:Client"
               and str(o["info"]["props"].get("application.process.id")) == sys.argv[2]}
    match = lambda p: p.get("client.id") in clients
else:
    match = lambda p: str(p.get(sys.argv[2])) == sys.argv[3]
for o in dump:
    if o.get("type") == "PipeWire:Interface:Node" and match(o["info"]["props"]):
        print(o["id"])
