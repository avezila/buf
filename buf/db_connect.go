package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/globalsign/mgo"
	"github.com/go-pg/pg"
	"github.com/jackc/pgx"
	"github.com/jackc/pgx/log/logrusadapter"
	"github.com/sirupsen/logrus"
)

func MognoConnect() (*mgo.Session, error) {
	session, err := mgo.Dial(os.Getenv("MONGODB_HOST"))
	if err != nil {
		return nil, err
	}
	return session, nil
	// Optional. Switch the session to a monotonic behavior.
	// session.SetMode(mgo.Monotonic, true)
}

func PgxPool(queries []*pgx.PreparedStatement) (pool *pgx.ConnPool, err error) {

	logger := logrusadapter.NewLogger(logrus.New())
	interval := time.Second * 1
	until := time.Now().Add(time.Minute * 5)
	firstStart := true
	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "postgres:5432"
	}
	port, err := strconv.Atoi(strings.Split(host, ":")[1])
	if err != nil {
		return nil, err
	}
	conf := pgx.ConnPoolConfig{
		ConnConfig: pgx.ConnConfig{
			Host:     strings.Split(host, ":")[0],
			Port:     uint16(port),
			Database: os.Getenv("DB_NAME"),
			User:     os.Getenv("DB_USER"),
			Password: os.Getenv("DB_PASS"),
			LogLevel: pgx.LogLevelWarn,
			Logger:   logger,
		},
	}
	if conf.MaxConnections, err = strconv.Atoi(os.Getenv("DB_MAX_CONNECTIONS")); err != nil {
		conf.MaxConnections = 50
	}
	for {
		if pool, err = pgx.NewConnPool(conf); err != nil {
			if time.Now().After(until) {
				return nil, err
			}
			log.Print("waiting postgres...", err.Error())
			time.Sleep(interval)
			continue
		}
		if firstStart {
			pool.Close()
			firstStart = false
			time.Sleep(time.Second * 2)
			continue
		}
		break
	}

	if err := PgPrepareAll(pool, queries); err != nil {
		return nil, err
	}
	return pool, nil
}

func PgPrepareAll(pool *pgx.ConnPool, queries []*pgx.PreparedStatement) error {
	for _, query := range queries {
		if err := PgPrepareEx(pool, query); err != nil {
			return err
		}
	}
	return nil
}

func PgPrepareEx(pool *pgx.ConnPool, ps *pgx.PreparedStatement) error {
	_, err := pool.PrepareEx(context.TODO(), ps.Name, ps.SQL, &pgx.PrepareExOptions{ParameterOIDs: ps.ParameterOIDs})
	return err
}

func PgConnect() *pg.DB {
	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "postgres:5432"
	}
	conf := &pg.Options{
		Addr:       host,
		User:       os.Getenv("DB_USER"),
		Password:   os.Getenv("DB_PASS"),
		Database:   os.Getenv("DB_NAME"),
		MaxRetries: 3,
	}
	if maxConnections, err := strconv.Atoi(os.Getenv("DB_MAX_CONNECTIONS")); err != nil {
		conf.PoolSize = 0
	} else {
		conf.PoolSize = maxConnections
	}

	db := pg.Connect(conf)
	db.OnQueryProcessed(func(event *pg.QueryProcessedEvent) {
		query, err := event.FormattedQuery()
		if err != nil {
			panic(err)
		}

		log.Printf("pg: %s %s", time.Since(event.StartTime), query)
	})
	return db
}
