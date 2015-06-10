echo 3 | sudo tee /proc/sys/vm/drop_caches

sudo \
SNEAKYNOTE_PORT=443 \
SNEAKYNOTE_CERTS=/home/sneakynote/src/github.com/brianhempel/sneakynote.com/sneakynote.com.certs \
SNEAKYNOTE_PRIVATE_KEY=/home/sneakynote/src/github.com/brianhempel/sneakynote.com/sneakynote.com.key \
/home/sneakynote/src/github.com/brianhempel/sneakynote.com/sneakynote.com
