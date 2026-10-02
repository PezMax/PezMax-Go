#!/bin/sh
set -eu

scenario="$1"
control=/test-control
sample() {
    read uptime _ < /proc/uptime
    # A single JSON row contains both the root TBF and its fair-queue child.
    printf '%s|%s\n' "$uptime" "$(tc -j -s qdisc show dev eth0)" >> "$control/$scenario-tc.samples"
}
sample
touch "$control/$scenario-monitor.ready"
while [ ! -f "$control/$scenario.start" ]; do sleep 0.05; done
while [ ! -f "$control/$scenario.stop" ]; do
    sample
    sleep 0.1
done
sample
touch "$control/$scenario-monitor.done"
