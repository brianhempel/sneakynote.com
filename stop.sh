sudo killall sneakynote.com && echo "sneakynote.com stopped"
sudo kill `cat free_memory_maximizer.sh.pid` 2> /dev/null && rm free_memory_maximizer.sh.pid 2> /dev/null
sudo kill `cat periodically_renew_certs.sh.pid` 2> /dev/null && rm periodically_renew_certs.sh.pid 2> /dev/null
