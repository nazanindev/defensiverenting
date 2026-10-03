#!/bin/zsh
# fileprops.sh <props.json>: triage check, then propose only on "0 problems".
cd /Users/nazimi/Dev/defensiverenting
out=$(bin/prod triage check "$1" 2>&1)
echo "$out" | grep -v "headless\|Quote monitor" | tail -6
if echo "$out" | tail -1 | grep -q " 0 problems"; then
  bin/prod propose -file "$1" -by "triage agent" 2>&1 | tail -2
else
  echo "NOT FILED"
fi
