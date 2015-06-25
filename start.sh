./stop.sh 2> /dev/null

./gzip_assets.sh

sudo sh -c "SNEAKYNOTE_PORT=443 \
SNEAKYNOTE_CERTS=/home/sneakynote/src/github.com/brianhempel/sneakynote.com/sneakynote.com.certs \
SNEAKYNOTE_PRIVATE_KEY=/home/sneakynote/src/github.com/brianhempel/sneakynote.com/sneakynote.com.key \
/home/sneakynote/src/github.com/brianhempel/sneakynote.com/sneakynote.com >> /home/sneakynote/src/github.com/brianhempel/sneakynote.com/public/log.txt 2>&1 &" && echo "sneakynote.com started"

sudo ./free_memory_maximizer.sh &
echo $! > free_memory_maximizer.sh.pid
