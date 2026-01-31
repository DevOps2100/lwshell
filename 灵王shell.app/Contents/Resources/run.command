#!/bin/bash
BINDIR="$(cd "$(dirname "$0")/../MacOS" && pwd)"
EXEC="$BINDIR/lwshell"

"$EXEC" --http=:21008 &
PID=$!
sleep 1.5
open "http://127.0.0.1:21008"
wait $PID
