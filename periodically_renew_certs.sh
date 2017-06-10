#!/usr/bin/env bash

while true; do

  sleep 1

  # https://stackoverflow.com/a/1401541
  # If certificates haven't been modified in more than 80 days, try to renew.
  if [ "$(( $(date +"%s") - $(stat -L -c "%Z" "letsencrypt/config/live/sneakynote.com/fullchain.pem") ))" -gt "6912000" ]; then

    # Ensure no outstanding notes.
    if [ "$(ls /tmp/sneakynote_store | wc -l)" -eq "4" ]; then
      echo "Renewing certificate"
      mkdir -p letsencrypt/config
      mkdir -p letsencrypt/work
      mkdir -p letsencrypt/logs
      certbot certonly --force-renewal --noninteractive --config-dir letsencrypt/config --work-dir letsencrypt/work --logs-dir letsencrypt/logs --webroot --webroot-path public/ -d sneakynote.com

      # Successfull renewal?
      if [ "$(( $(date +"%s") - $(stat -L -c "%Z" "letsencrypt/config/live/sneakynote.com/fullchain.pem") ))" -le "6912000" ]; then
        # Start script will kill this script.
        sudo ./start.sh &
      fi
    fi
  fi

  sleep 600
done

