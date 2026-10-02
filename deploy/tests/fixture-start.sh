#!/bin/sh
set -eu
dd if=/dev/zero of=/fixtures/payload.bin bs=1048576 count=2 2>/dev/null
exec nginx -g 'daemon off;'
