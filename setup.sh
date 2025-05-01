#!/bin/sh

docker network inspect shared-network > /dev/null 2>&1

exit_code=$?

if [ $exit_code -eq 0 ]; then
  echo "Network exists."
else
  echo "Network does not exist."
  docker network create shared-network
fi

