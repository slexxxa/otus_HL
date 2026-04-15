package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
)

var dbWrite *sql.DB
var dbRead *sql.DB
var rdb *redis.Client

func initDB() {
	writeConn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s",
		env("PGUSER", "postgres"),
		env("PGPASSWORD", "postgres"),
		env("PGHOST_WRITE", "10.169.44.8"),
		env("PGPORT_WRITE", "5000"),
		env("PGDBNAME", "auth"),
	)

	readConn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s",
		env("PGUSER", "postgres"),
		env("PGPASSWORD", "postgres"),
		env("PGHOST_READ", "10.169.44.8"),
		env("PGPORT_READ", "5001"),
		env("PGDBNAME", "auth"),
	)

	var err error
	dbWrite, err = sql.Open("pgx", writeConn)
	if err != nil {
		log.Fatal(err)
	}

	dbRead, err = sql.Open("pgx", readConn)
	if err != nil {
		log.Fatal(err)
	}
}

func env(k, def string) string {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	return v
}

func initRedis() {
	rdb = redis.NewClient(&redis.Options{
		Addr: env("REDISCONN", "10.169.44.8:6379"),
	})
}
