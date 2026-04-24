#!/usr/bin/env bash
set -e

apt update
apt install -y curl jq vim redis-tools

echo "start redis migrate"

cat /initdb/dialog.lua | redis-cli -h redis -x FUNCTION LOAD REPLACE

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

### запуск скрипта на первой worker node ###
echo "Запуск миграции для первого воркера"
echo "|"
echo "|"
echo "|"
echo "|"
echo "|"
echo "|"
echo "|"
echo "|"
echo "|"
LEADER=""
NODES=("patroni-w1" "patroni-w2")

until [ -n "$LEADER" ]; do
  get_leader
  echo "Leader: $LEADER"
  sleep 2
done

echo "worker detected: $LEADER"

export PGHOST=$LEADER
w1=$LEADER
export PGUSER=postgres
export PGPASSWORD=postgres
export PGPORT=5432

echo "Waiting postgres..."

until pg_isready; do
  sleep 2
done

echo "Running init_w.sql"

psql -f /initdb/init_w.sql

echo "|"
echo "|"
echo "|"
echo "|"
echo "|"
echo "|"
echo "|"
echo "|"
echo "|"
### запуск скрипта на второй worker node ###
echo "Запуск миграции для второго воркера"
echo "|"
echo "|"
echo "|"
echo "|"
echo "|"
echo "|"
echo "|"
echo "|"
echo "|"
NODES=("patroni-w3" "patroni-w4")
LEADER=""
until [ -n "$LEADER" ]; do
  get_leader
  echo "Leader: $LEADER"
  sleep 2
done

echo "worker2 detected: $LEADER"
export PGHOST=$LEADER
w2=$LEADER
echo "Waiting postgres..."
until pg_isready; do
  sleep 2
done
echo "Running init_w.sql"
psql -f /initdb/init_w.sql

echo "|"
echo "|"
echo "|"
echo "|"
echo "|"
echo "|"
echo "|"
echo "|"
echo "|"
### запуск скрипта на координаторе node ###
echo "Запуск миграции для координатора"
echo "|"
echo "|"
echo "|"
echo "|"
echo "|"
echo "|"
echo "|"
echo "|"
echo "|"
NODES=("patroni-c1" "patroni-c2")
LEADER=""
until [ -n "$LEADER" ]; do
  get_leader
  echo "Leader: $LEADER"
  sleep 2
done
echo "leader detected: $LEADER"
export PGHOST=$LEADER
c=$LEADER
echo "Waiting postgres..."
until pg_isready; do
  sleep 2
done
echo "Running init.sql"
psql -v w1=${w1} -v w2=${w2} -v c=${c} -f /initdb/init.sql

echo "|"
echo "|"
echo "|"
echo "|"
echo "|"
echo "|"
echo "|"
echo "|"
echo "|"

echo "Init completed"
