#!/usr/bin/env bash
set -e

apt update
apt install -y curl jq vim

NODES=("patroni1" "patroni2" "patroni3")
API_PORT=8008

echo "Searching for Patroni leader..."

get_leader() {
  for node in "${NODES[@]}"; do
    role=$(curl -s http://$node:$API_PORT | jq .role )
    echo node is $node
    if [ "$role" = '"primary"' ]; then
      echo "$node"
      LEADER=$node
      return
    fi
  done
}

LEADER=""

until [ -n "$LEADER" ]; do
  get_leader
  echo "Leader: $LEADER"
  sleep 2
done

echo "Leader detected: $LEADER"

export PGHOST=$LEADER
export PGUSER=postgres
export PGPASSWORD=postgres
export PGPORT=5432

echo "Waiting postgres..."

until pg_isready; do
  sleep 2
done

echo "Running init.sql"

psql -f /initdb/init.sql

echo "Init completed"
