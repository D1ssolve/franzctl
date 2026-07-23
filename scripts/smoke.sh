#!/bin/sh
set -eu

binary="${1:-./bin/franzctl}"
broker="${FRANZCTL_BROKERS:-localhost:9092}"
topic="franzctl-smoke-$$"

"$binary" topic create \
  --broker "$broker" \
  --topic "$topic" \
  --partitions 1 \
  --replication-factor 1

"$binary" produce \
  --broker "$broker" \
  --topic "$topic" \
  --key smoke \
  --value '{"ok":true}' \
  --value-codec json

output=$("$binary" consume \
  --broker "$broker" \
  --topic "$topic" \
  --partition 0 \
  --from beginning \
  --max 1 \
  --follow=false \
  --key-codec string \
  --value-codec json)

printf '%s\n' "$output" | grep -q '"ok":true'
printf '%s\n' "smoke test passed"
