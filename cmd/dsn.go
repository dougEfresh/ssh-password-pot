package cmd

import (
	log "github.com/Sirupsen/logrus"
	"gopkg.in/dougEfresh/dbr.v2"
	"strings"
)

const (
	eventTable    = "event"
	eventGeoTable = "vw_event"
	geoTable      = "geo"
)

func loadDSN(dsn string) *dbr.Connection {
	var db *dbr.Connection
	var err error
	if strings.Contains(dsn, "postgres") {
		log.Debug("Using pq driver")
		db, err = dbr.Open("postgres", dsn, defaultDbEventLogger)
	} else {
		log.Debug("Using mysql driver")
		db, err = dbr.Open("mysql", dsn, defaultDbEventLogger)
	}

	if err != nil {
		panic(err)
	}
	err = db.Ping()
	if err != nil {
		panic(err)
	}

	return db
}
