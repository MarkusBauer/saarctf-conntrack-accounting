package main

import (
	"github.com/ti-mo/conntrack"
	"log"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type AccountingEntry struct {
	packetsSrcToDst, bytesSrcToDst uint64
	packetsDstToSrc, bytesDstToSrc uint64
	connectionCount                int
	connectionTime                 int64
	openConnections                int
}

var AccountingTable = make(map[string]*AccountingEntry)

func ConvertIp(ip netip.Addr) net.IP {
	b := ip.As4()
	return b[:]
}

func AccountingKey(flow *conntrack.Flow) string {
	proto := ProtoLookup(flow.TupleOrig.Proto.Protocol)
	s := proto + ","
	s += ConvertIp(flow.TupleOrig.IP.SourceAddress).Mask(SourceGroupMask).String() + ","
	s += ConvertIp(flow.TupleOrig.IP.DestinationAddress).Mask(DestGroupMask).String() + ","
	if PortIsInteresting(proto, flow.TupleOrig.Proto.DestinationPort) {
		s += strconv.FormatUint(uint64(flow.TupleOrig.Proto.DestinationPort), 10)
	} else {
		s += "-1"
	}
	return s
}

func getOrCreateAccountingTableEntry(key string) *AccountingEntry {
	entry := AccountingTable[key]
	if entry == nil {
		entry = &AccountingEntry{}
		AccountingTable[key] = entry
	}
	return entry
}

func AccountTraffic(info *ConnectionInfo) {
	// Is there anything to account?
	if info.packetsSrcToDst == info.packetsSrcToDstAccounted && info.bytesSrcToDst == info.bytesSrcToDstAccounted {
		if info.packetsDstToSrc == info.packetsDstToSrcAccounted && info.bytesDstToSrc == info.bytesDstToSrcAccounted {
			return
		}
	}
	// Account data and reset connection
	entry := getOrCreateAccountingTableEntry(info.key)
	if info.packetsSrcToDst > info.packetsSrcToDstAccounted {
		entry.packetsSrcToDst += info.packetsSrcToDst - info.packetsSrcToDstAccounted
		info.packetsSrcToDstAccounted = info.packetsSrcToDst
	}
	if info.packetsDstToSrc > info.packetsDstToSrcAccounted {
		entry.packetsDstToSrc += info.packetsDstToSrc - info.packetsDstToSrcAccounted
		info.packetsDstToSrcAccounted = info.packetsDstToSrc
	}
	if info.bytesSrcToDst > info.bytesSrcToDstAccounted {
		entry.bytesSrcToDst += info.bytesSrcToDst - info.bytesSrcToDstAccounted
		info.bytesSrcToDstAccounted = info.bytesSrcToDst
	}
	if info.bytesDstToSrc > info.bytesDstToSrcAccounted {
		entry.bytesDstToSrc += info.bytesDstToSrc - info.bytesDstToSrcAccounted
		info.bytesDstToSrcAccounted = info.bytesDstToSrc
	}
}

func AccountConnectionClose(info *ConnectionInfo) {
	if !info.connectionTrackingDisabled {
		info.connectionTrackingDisabled = true
		duration := time.Now().Sub(info.start).Milliseconds()
		entry := getOrCreateAccountingTableEntry(info.key)
		entry.connectionCount += 1
		entry.connectionTime += duration
	}
}

func AccountOpenConnection(info *ConnectionInfo) {
	entry := getOrCreateAccountingTableEntry(info.key)
	entry.openConnections += 1
}

func FlushAccountingTableToOutput(timestamp time.Time) {
	start := time.Now()
	size := len(AccountingTable)

	var f *os.File
	var err error
	if OutputFolder != "" {
		fname := filepath.Join(OutputFolder, "traffic_"+timestamp.Format("2006-01-02T15_04_05")+".csv")
		f, err = os.OpenFile(fname, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatal("Open csv file", err)
		}
		defer f.Close()
	}

	for key, entry := range AccountingTable {
		// format:
		// time,proto,src,dst,port,packets_src,packets_dst,bytes_src,bytes_dst,connection_count,connection_time,open_connections
		var line strings.Builder
		line.WriteString(strconv.FormatInt(timestamp.UnixNano(), 10))
		line.WriteString(",")
		line.WriteString(key)
		line.WriteString(",")
		line.WriteString(strconv.FormatUint(entry.packetsSrcToDst, 10))
		line.WriteString(",")
		line.WriteString(strconv.FormatUint(entry.packetsDstToSrc, 10))
		line.WriteString(",")
		line.WriteString(strconv.FormatUint(entry.bytesSrcToDst, 10))
		line.WriteString(",")
		line.WriteString(strconv.FormatUint(entry.bytesDstToSrc, 10))
		line.WriteString(",")
		line.WriteString(strconv.Itoa(entry.connectionCount))
		line.WriteString(",")
		line.WriteString(strconv.FormatInt(entry.connectionTime, 10))
		if TrackOpenConnections {
			line.WriteString(",")
			line.WriteString(strconv.Itoa(entry.openConnections))
		}
		line.WriteString("\n")
		_, err := Output.WriteString(line.String())
		if err != nil {
			log.Fatal("Output write error: ", err)
		}
		if f != nil {
			_, err := f.WriteString(line.String())
			if err != nil {
				log.Fatal("Output write error (file): ", err)
			}
		}
	}
	// Clear accounting table
	AccountingTable = make(map[string]*AccountingEntry)

	log.Println("[Output] wrote", size, "entries in", time.Now().Sub(start).Milliseconds(), "ms")
}
