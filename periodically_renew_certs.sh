#!/usr/bin/env bash

while true; do

  # https://stackoverflow.com/a/1401541
  # If private key hasn't been modified in more than 80 days, try to renew.
  if [ "$(( $(date +"%s") - $(stat -L -c "%Z" "letsencrypt/config/live/sneakynote.com/fullchain.pem") ))" -gt "6912000" ]; then
    if [ "$(ls /tmp/sneakynote_store | wc -l)" -eq "4" ]; then
      echo "Renewing certificate"
      mkdir -p letsencrypt/config
      mkdir -p letsencrypt/work
      mkdir -p letsencrypt/logs
      # Start script will kill this script
      certbot certonly --noninteractive --config-dir letsencrypt/config --work-dir letsencrypt/work --logs-dir letsencrypt/logs --webroot --webroot-path public/ -d sneakynote.com && sudo ./start.sh &
    fi
  fi

  sleep 600
done

