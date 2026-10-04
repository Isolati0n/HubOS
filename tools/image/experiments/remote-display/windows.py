"""Print app-id, title and size of every window from `swaymsg -t get_tree` (stdin). The sway stand-in for what driftwm reports."""
import json,sys
def walk(n):
    if n.get("app_id"): print(n["app_id"], repr(n["name"]), n["rect"]["width"],"x",n["rect"]["height"], "floating" if n["type"]=="floating_con" else "")
    for c in n.get("nodes",[])+n.get("floating_nodes",[]): walk(c)
walk(json.load(sys.stdin))
