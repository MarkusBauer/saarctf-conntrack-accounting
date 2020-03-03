package main

import (
	"github.com/ti-mo/conntrack"
	"log"
	"strings"
	"time"
)

type AccountingEntry struct {
	packetsSrcToDst, bytesSrcToDst uint64
	packetsDstToSrc, bytesDstToSrc uint64
	connectionCount                int
	connectionTime                 int64
}

var AccountingTable = make(map[string]*AccountingEntry)

func getOrCreateAccountingTableEntry(flow *conntrack.Flow) *AccountingEntry {
	key := flow.TupleOrig.IP.SourceAddress.String() + "," + flow.TupleOrig.IP.DestinationAddress.String() + "," + string(flow.TupleOrig.Proto.DestinationPort)
	entry := AccountingTable[key]
	if entry == nil {
		entry = &AccountingEntry{}
		AccountingTable[key] = entry
	}
	return entry
}

func AccountTraffic(flow *conntrack.Flow, info *ConnectionInfo) {
	// Is there anything to account?
	if info.packetsSrcToDst == info.packetsSrcToDstAccounted && info.bytesSrcToDst == info.bytesSrcToDstAccounted {
		if info.packetsDstToSrc == info.packetsDstToSrcAccounted && info.bytesDstToSrc == info.bytesDstToSrcAccounted {
			return
		}
	}
	// Account data
	entry := getOrCreateAccountingTableEntry(flow)
	entry.packetsSrcToDst += info.packetsSrcToDst - info.packetsSrcToDstAccounted
	entry.packetsDstToSrc += info.packetsDstToSrc - info.packetsDstToSrcAccounted
	entry.bytesSrcToDst += info.bytesSrcToDst - info.bytesSrcToDstAccounted
	entry.bytesDstToSrc += info.bytesDstToSrc - info.bytesDstToSrcAccounted
	// Reset connection
	info.packetsSrcToDstAccounted = info.packetsSrcToDst
	info.packetsDstToSrcAccounted = info.packetsDstToSrc
	info.bytesSrcToDstAccounted = info.bytesSrcToDst
	info.bytesDstToSrcAccounted = info.bytesDstToSrc
}

func AccountConnectionClose(flow *conntrack.Flow, info *ConnectionInfo) {
	info.closed = true
	duration := time.Now().Sub(info.start).Milliseconds()
	entry := getOrCreateAccountingTableEntry(flow)
	entry.connectionCount += 1
	entry.connectionTime += duration
}

func FlushAccountingTableToOutput() {
	for key, entry := range AccountingTable {
		var line strings.Builder
		line.WriteString(key)
		line.WriteString(",")
		line.WriteString(string(entry.connectionCount))
		line.WriteString(",")
		line.WriteString(string(entry.connectionTime))
		line.WriteString(",")
		line.WriteString(string(entry.packetsSrcToDst))
		line.WriteString(",")
		line.WriteString(string(entry.packetsDstToSrc))
		line.WriteString(",")
		line.WriteString(string(entry.bytesSrcToDst))
		line.WriteString(",")
		line.WriteString(string(entry.bytesDstToSrc))
		line.WriteString("\n")
		_, err := Output.WriteString(line.String())
		if err != nil {
			log.Fatal("Output write", err)
		}
	}
	// Flush output
	err := Output.Sync()
	if err != nil {
		log.Fatal("Output sync", err)
	}
	// Clear accounting table
	AccountingTable = make(map[string]*AccountingEntry)
}
