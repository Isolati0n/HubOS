#!/bin/bash
# waitfor.sh LOG [max seconds]: returns when the run log has its final line
L=$1; M=${2:-3000}
for i in $(seq 1 $((M/10))); do
  grep -q "^H# end" "$L" 2>/dev/null && { echo finished; exit 0; }
  sleep 10
done
echo timeout
