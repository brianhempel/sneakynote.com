sudo killall sneakynote.com 2> /dev/null && echo "sneakynote.com stopped"
sudo kill `cat free_memory_maximizer.sh.pid 2> /dev/null` 2> /dev/null && rm free_memory_maximizer.sh.pid
