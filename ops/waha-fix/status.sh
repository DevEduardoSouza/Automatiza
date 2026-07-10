#!/bin/bash
# One-line status for the purge waiter and quick checks.
R=$(pgrep -f "waha-fix/purge.py" | wc -l)
P=$(python3 -c 'import json;print(len(json.load(open("/root/waha-fix/purge_state.json"))))' 2>/dev/null || echo 0)
DF=$(df -h / | awk 'NR==2{print $4" free "$5}')
echo "R=$R P=$P/520 $DF"
