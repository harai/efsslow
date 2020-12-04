#!/bin/bash

set -eu
trap 'kill $(jobs -p)' EXIT
if [[ $# != 1 ]]; then
  echo "Usage: $0 <file_path>" >&2
  exit 0
fi
gcc ./efs_open.c -o ./efs_open >&2
./efs_open "$1" &
echo "$!"
./efs_open "$1" &
echo "$!"
sleep 60
kill %1
kill %2
