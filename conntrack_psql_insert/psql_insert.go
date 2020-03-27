package main

import (
	"flag"
	"log"
)

func main() {
	watchFolder := flag.String("watch", "", "Watch this folder for incoming csv's")
	flag.Parse()

	db := Database{}
	err := db.Open("markus", "123456789", "localhost", "saarctf_2")
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
