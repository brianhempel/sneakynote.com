#!/usr/bin/env bash

# Need to run with sudo

while true; do
  echo 3 | /proc/sys/vm/drop_caches
  sleep 600
done
