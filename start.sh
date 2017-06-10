./stop.sh 2> /dev/null

./gzip_assets.sh

sudo sh -c "SNEAKYNOTE_PORT=443 \
SNEAKYNOTE_CERTS=/home/sneakynote/src/github.com/brianhempel/sneakynote.com/letsencrypt/config/live/sneakynote.com/fullchain.pem \
SNEAKYNOTE_PRIVATE_KEY=/home/sneakynote/src/github.com/brianhempel/sneakynote.com/letsencrypt/config/live/sneakynote.com/privkey.pem \
/home/sneakynote/src/github.com/brianhempel/sneakynote.com/sneakynote.com >> /home/sneakynote/src/github.com/brianhempel/sneakynote.com/log.txt 2>&1 &" && echo "sneakynote.com started"

sudo ./periodically_renew_certs.sh &
echo $! > periodically_renew_certs.sh.pid

sudo ./free_memory_maximizer.sh &
echo $! > free_memory_maximizer.sh.pid
