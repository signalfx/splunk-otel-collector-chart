#!/bin/bash

catalina.sh run &
pid=$!
loop=0

function stop()
{
  kill $pid
  loop=1
  echo "Good bye"
}

trap stop SIGINT

while [ $loop -eq 0 ]; do
  curl -s http://localhost:8080 > /dev/null
  sleep 1
done
