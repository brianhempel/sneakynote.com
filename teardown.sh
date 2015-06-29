./stop.sh 2> /dev/null

sudo sh -c "/home/sneakynote/src/github.com/brianhempel/sneakynote.com/sneakynote.com teardown >> /home/sneakynote/src/github.com/brianhempel/sneakynote.com/log.txt 2>&1 &" && echo "sneakynote.com datastore torn down"
