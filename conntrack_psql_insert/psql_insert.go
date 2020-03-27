package main

import (
	"flag"
	"log"
)

func main() {
	hostname := flag.String("host", "localhost", "Postgresql hostname")
	database := flag.String("db", "", "Postgresql database")
	username := flag.String("user", "", "Postgresql username")
	passwd := flag.String("pass", "", "Postgresql password")
	watchFolder := flag.String("watch", "", "Watch this folder for incoming csv's")
	flag.Parse()

	db := Database{}
	err := db.Open(*username, *passwd, *hostname, *database)
	if err != nil {
		log.Fatal("DB open:", err)
	}

	// create table
	db.CreateTable()

	for _, fname := range flag.Args() {
		db.InsertCSV(fname)
	}

	if watchFolder != nil && *watchFolder != "" {
		// TODO
	}
}
