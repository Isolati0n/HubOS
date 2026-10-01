#!/bin/sh
# FAKE viewer that opens TWO windows at once, to test the "cannot tell which
# window is whose" path. Used with testdata/viewers/ambiguous.toml.
foot --app-id=two-a --title="Two A" -- sleep infinity &
foot --app-id=two-b --title="Two B" -- sleep infinity &
wait
