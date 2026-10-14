#!/bin/sh
set -eu

fail() {
    echo "edge egress: $*" >&2
    exit 1
}

rate=${KADMIN_EDGE_EGRESS_RATE-8mbit}
burst=${KADMIN_EDGE_EGRESS_BURST_BYTES-32768}

# Explicit bit/s units avoid confusing tc's rate with Nginx's byte/s values.
echo "$rate" | grep -Eq '^[1-9][0-9]*(bit|kbit|mbit|gbit)$' \
    || fail 'KADMIN_EDGE_EGRESS_RATE must be a positive integer with bit/kbit/mbit/gbit units'
echo "$burst" | grep -Eq '^[1-9][0-9]*$' \
    || fail 'KADMIN_EDGE_EGRESS_BURST_BYTES must be an integer between 1500 and 1048576'
[ "$burst" -ge 1500 ] && [ "$burst" -le 1048576 ] \
    || fail 'KADMIN_EDGE_EGRESS_BURST_BYTES must be between 1500 and 1048576'
command -v tc >/dev/null 2>&1 || fail 'tc is missing; use Dockerfile.edge'

# One kernel queue on the edge's only external interface covers every worker,
# client, token and response path, including IPv4/IPv6 and TCP retransmissions.
# Keep the capability limited to this container's network namespace.
tc qdisc replace dev eth0 root handle 1: tbf \
    rate "$rate" burst "$burst" latency 250ms \
    || fail 'cannot install the shared bandwidth pool; NET_ADMIN and kernel TBF support are required'

# TBF supplies the aggregate cap; fq_codel shares queued packets among flows.
# A child qdisc replaces TBF's own queue, so bound its memory explicitly.
tc qdisc replace dev eth0 parent 1:1 handle 10: fq_codel \
    limit 256 memory_limit 262144 \
    || fail 'cannot install the bounded fair queue; kernel fq_codel support is required'

echo "edge egress: shared eth0 budget=$rate, burst=$burst bytes"
tc qdisc show dev eth0
