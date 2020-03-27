package main

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type StatsEntry struct {
	time            time.Time
	src             string
	dst             string
	proto           string
	port            int
	srcPackets      int64
	srcBytes        int64
	dstPackets      int64
	dstBytes        int64
	connectionTimes int
	connectionCount int
	openConnections int
}

type Database struct {
	db *sql.DB
}

func (database *Database) Open(username, passwd, host, dbname string) error {
	var err error
	database.db, err = sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", username, passwd, host, dbname))
	if err != nil {
		return err
	}
	database.db.SetMaxIdleConns(10)
	database.db.SetMaxOpenConns(10)
	database.db.SetConnMaxLifetime(0)
	return nil
}

func (database *Database) Close() {
	_ = database.db.Close()
}

func (database *Database) CreateTable() {
	_, err := database.db.Exec(`
CREATE TABLE IF NOT EXISTS vpn_traffic (
	id serial PRIMARY KEY,
	time timestamp with time zone NOT NULL,
	src varchar(16) NOT NULL,
	dst varchar(16) NOT NULL,
	proto varchar(4) NOT NULL,
	port INT NOT NULL,
	src_packets BIGINT NOT NULL,
	src_bytes BIGINT NOT NULL,
	dst_packets BIGINT NOT NULL,
	dst_bytes BIGINT NOT NULL,
	connection_times INT NOT NULL,
	connection_count INT NOT NULL,
	open_connections INT NOT NULL,
	UNIQUE(time, src, dst, proto, port)
);
CREATE INDEX IF NOT EXISTS vpn_traffic_time_idx ON vpn_traffic ("time" DESC);
CREATE INDEX IF NOT EXISTS vpn_traffic_src_idx ON vpn_traffic ("src");
CREATE INDEX IF NOT EXISTS vpn_traffic_dst_idx ON vpn_traffic ("dst");
CREATE INDEX IF NOT EXISTS vpn_traffic_proto_port_idx ON vpn_traffic ("proto", "port");
`)
	if err != nil {
		log.Fatal(err)
	}
}

func readCSV(fname string) []StatsEntry {
	csvfile, err := os.Open(fname)
	if err != nil {
		log.Fatal("Couldn't open the csv file", err)
	}

	entries := make([]StatsEntry, 0, 2048)

	r := csv.NewReader(csvfile)
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		if len(record) < 2 {
			continue
		}
		// format: time,proto,src,dst,port,packets_src,packets_dst,bytes_src,bytes_dst,connection_count,connection_time,open_connections
		t, err := strconv.ParseInt(record[0], 10, 64)
		if err != nil {
			log.Fatal("Invalid t: ", err)
		}
		port, err := strconv.ParseInt(record[4], 10, 32)
		if err != nil {
			log.Fatal("Invalid port: ", err)
		}
		srcPackets, err := strconv.ParseInt(record[5], 10, 64)
		if err != nil {
			log.Fatal("Invalid src_packets: ", err)
		}
		srcBytes, err := strconv.ParseInt(record[7], 10, 64)
		if err != nil {
			log.Fatal("Invalid src_bytes: ", err)
		}
		dstPackets, err := strconv.ParseInt(record[6], 10, 64)
		if err != nil {
			log.Fatal("Invalid dst_packets: ", err)
		}
		dstBytes, err := strconv.ParseInt(record[8], 10, 64)
		if err != nil {
			log.Fatal("Invalid dst_bytes: ", err)
		}
		connectionTimes, err := strconv.ParseInt(record[10], 10, 32)
		if err != nil {
			log.Fatal("Invalid connection_times: ", err)
		}
		connectionCount, err := strconv.ParseInt(record[9], 10, 32)
		if err != nil {
			log.Fatal("Invalid connection_count: ", err)
		}
		openConnections := int64(0)
		if len(record) > 11 {
			openConnections, err = strconv.ParseInt(record[11], 10, 32)
			if err != nil {
				log.Fatal("Invalid open_connections: ", err)
			}
		}
		entries = append(entries, StatsEntry{
			time:            time.Unix(t/1000000000, t%1000000000),
			src:             record[2],
			dst:             record[3],
			proto:           record[1],
			port:            int(port),
			srcPackets:      srcPackets,
			srcBytes:        srcBytes,
			dstPackets:      dstPackets,
			dstBytes:        dstBytes,
			connectionTimes: int(connectionTimes),
			connectionCount: int(connectionCount),
			openConnections: int(openConnections),
		})
	}
	return entries
}

func (database *Database) InsertCSV(fname string) {
	start := time.Now()

	// Load CSV
	stats := readCSV(fname)

	// Save to database
	txn, err := database.db.Begin()
	if err != nil {
		log.Fatal(err)
	}

	//err = database.copyfrom(txn, stats)
	err = database.bulkInsert(txn, stats)
	if err != nil {
		log.Fatal(err)
	}

	err = txn.Commit()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Imported %d entries in %d ms\n", len(stats), time.Now().Sub(start).Milliseconds())
}

// COPY IN variant - not save if data is repeated
func (database *Database) copyfrom(txn *sql.Tx, stats []StatsEntry) error {
	stmt, _ := txn.Prepare(pq.CopyIn("vpn_traffic", "time", "src", "dst", "proto", "port", "src_packets", "src_bytes", "dst_packets", "dst_bytes", "connection_times", "connection_count", "open_connections"))
	for _, stat := range stats {
		_, err := stmt.Exec(stat.time, stat.src, stat.dst, stat.proto, stat.port, stat.srcPackets, stat.srcBytes, stat.dstPackets, stat.dstBytes, stat.connectionTimes, stat.connectionCount, stat.openConnections)
		if err != nil {
			log.Fatal(err)
		}
	}
	_, err := stmt.Exec()
	if err != nil {
		return err
	}
	err = stmt.Close()
	if err != nil {
		return err
	}
	return nil
}

// INSERT INTO variant - works always
func (database *Database) bulkInsert(tx *sql.Tx, unsavedRows []StatsEntry) error {
	valueStrings := make([]string, 0, 1000)
	valueArgs := make([]interface{}, 0, 12000)
	i := 0
	var prep *sql.Stmt
	var err error
	for _, row := range unsavedRows {
		if prep == nil {
			valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)", i*12+1, i*12+2, i*12+3, i*12+4, i*12+5, i*12+6, i*12+7, i*12+8, i*12+9, i*12+10, i*12+11, i*12+12))
		}
		valueArgs = append(valueArgs, row.time)
		valueArgs = append(valueArgs, row.src)
		valueArgs = append(valueArgs, row.dst)
		valueArgs = append(valueArgs, row.proto)
		valueArgs = append(valueArgs, row.port)
		valueArgs = append(valueArgs, row.srcPackets)
		valueArgs = append(valueArgs, row.srcBytes)
		valueArgs = append(valueArgs, row.dstPackets)
		valueArgs = append(valueArgs, row.dstBytes)
		valueArgs = append(valueArgs, row.connectionTimes)
		valueArgs = append(valueArgs, row.connectionCount)
		valueArgs = append(valueArgs, row.openConnections)
		i++
		if i == 500 { // bulk size
			if prep == nil {
				stmt := fmt.Sprintf("INSERT INTO vpn_traffic (\"time\", src, dst, proto, port, src_packets, src_bytes, dst_packets, dst_bytes, connection_times, connection_count, open_connections) VALUES %s ON CONFLICT DO NOTHING", strings.Join(valueStrings, ","))
				prep, err = tx.Prepare(stmt)
				if err != nil {
					return err
				}
			}
			_, err = prep.Exec(valueArgs...)
			if err != nil {
				return err
			}
			i = 0
			valueArgs = make([]interface{}, 0, 12000)
		}
	}
	if len(valueArgs) > 0 {
		stmt := fmt.Sprintf("INSERT INTO vpn_traffic (\"time\", src, dst, proto, port, src_packets, src_bytes, dst_packets, dst_bytes, connection_times, connection_count, open_connections) VALUES %s ON CONFLICT DO NOTHING", strings.Join(valueStrings[:len(valueArgs)/12], ","))
		_, err := database.db.Exec(stmt, valueArgs...)
		return err
	} else {
		return nil
	}
}
