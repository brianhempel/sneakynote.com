for path in ./public/* ; do
  if [ -f "$path" ] ; then
    (echo "$path" | egrep -i "\\.(jpe?g|gif|png|gz)$" > /dev/null) || (cat "$path" | gzip -9 > "$path.gz")
  fi
done

for path in ./public/**/* ; do
  if [ -f "$path" ] ; then
    (echo "$path" | egrep -i "\\.(jpe?g|gif|png|gz)$" > /dev/null) || (cat "$path" | gzip -9 > "$path.gz")
  fi
done
