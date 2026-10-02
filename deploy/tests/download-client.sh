#!/bin/sh
set -eu

scenario="$1"
client="$2"
url="$3"
control=/test-control
result="$control/$scenario-client-$client"
touch "$result.ready"
while [ ! -f "$control/$scenario.start" ]; do sleep 0.05; done

read start _ < /proc/uptime
status=0
wget -S --header="Authorization: Bearer test-token-$client" \
    -O "/tmp/$scenario-payload.bin" "$url" 2> "$result.log" || status=$?
read end _ < /proc/uptime
bytes=0
if [ -f "/tmp/$scenario-payload.bin" ]; then
    bytes=$(wc -c < "/tmp/$scenario-payload.bin")
fi
printf '%s|%s|%s|%s\n' "$status" "$bytes" "$start" "$end" > "$result.tmp"
mv "$result.tmp" "$result.done"
rm -f "/tmp/$scenario-payload.bin"
