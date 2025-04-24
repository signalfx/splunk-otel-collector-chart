#!/bin/bash

# Start the request loop in the background
./request-loop.sh > dev/null &
request_loop_pid=$!

function stop()
{
  kill $request_loop_pid
  echo "Good bye"
}

trap stop SIGINT

OTEL_EXPORTER_OTLP_PROTOCOL=grpc OTEL_LOG_LEVEL=DEBUG node index.js
