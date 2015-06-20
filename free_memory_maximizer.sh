#!/usr/bin/env bash

# Need to run with sudo

while true; do
  echo 3 | sudo tee /proc/sys/vm/drop_caches > /dev/null
  sleep 600
done
