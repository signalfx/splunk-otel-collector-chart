#!/bin/bash

node index.js &
nodepid=$!
loop=0

function stop()
{
  kill $nodepid
  loop=1
  echo "Good bye"
}

trap stop SIGINT

while [ $loop -eq 0 ]; do
  curl -s http://localhost:3000
  sleep 1
done
