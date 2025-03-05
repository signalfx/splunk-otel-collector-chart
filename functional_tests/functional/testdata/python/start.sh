#!/bin/bash

python3 web.py &
pypid=$!


loop=0

function stop()
{
  kill $pypid
  loop=1
  echo "Good bye"
}

trap stop SIGINT

while [ $loop -eq 0 ]; do
  curl http://localhost:5000 &
  sleep 1
done
