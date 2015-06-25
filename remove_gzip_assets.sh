for path in public/* ; do
  if [ -f "$path" ] ; then
    (echo "$path" | egrep -i "\\.(jpe?g|gif|png|gz)$" > /dev/null) || (rm "$path.gz")
  fi
done

for path in public/**/* ; do
  if [ -f "$path" ] ; then
    (echo "$path" | egrep -i "\\.(jpe?g|gif|png|gz)$" > /dev/null) || (rm "$path.gz")
  fi
done
