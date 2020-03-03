package main

import (
	"github.com/mdlayher/netlink"
	"github.com/ti-mo/conntrack"
	"github.com/ti-mo/netfilter"
	"log"
	"time"
)

type ConnectionInfo struct {
	packetsSrcToDst, bytesSrcToDst                   uint64
	packetsDstToSrc, bytesDstToSrc                   uint64
	packetsSrcToDstAccounted, bytesSrcToDstAccounted uint64
	packetsDstToSrcAccounted, bytesDstToSrcAccounted uint64
	closed                                           bool
	start                                            time.Time
}

var connections = make(map[uint32]*ConnectionInfo)

func handleNewFlow(flow *conntrack.Flow) {
	connections[flow.ID] = &ConnectionInfo{start: time.Now()}
}

func handleDestroyFlow(flow *conntrack.Flow) {
	if info, ok := connections[flow.ID]; ok {
		delete(connections, flow.ID)
		if flow.CountersOrig.Packets != 0 && flow.CountersOrig.Bytes != 0 {
			info.packetsSrcToDst = flow.CountersOrig.Packets
			info.bytesSrcToDst = flow.CountersOrig.Bytes
		}
		if flow.CountersReply.Packets != 0 && flow.CountersReply.Bytes != 0 {
			info.packetsDstToSrc = flow.CountersReply.Packets
			info.bytesDstToSrc = flow.CountersReply.Bytes
		}
		AccountTraffic(flow, info)
		if !info.closed {
			AccountConnectionClose(flow, info)
		}
	}
}

func handleTerminateFlow(flow *conntrack.Flow) {
	if info, ok := connections[flow.ID]; ok {
		if !info.closed {
			AccountConnectionClose(flow, info)
		}
	}
}

func handleConntrackEvent(event conntrack.Event) {
	switch event.Type {
	case conntrack.EventNew:
		handleNewFlow(event.Flow)
	case conntrack.EventDestroy:
		handleDestroyFlow(event.Flow)
	case conntrack.EventUpdate:
		// Check if we know this flow and should terminate it
		state := event.Flow.ProtoInfo.TCP.State
		if state == TCP_CONNTRACK_CLOSE_WAIT || state == TCP_CONNTRACK_LAST_ACK || state == TCP_CONNTRACK_CLOSE {
			handleTerminateFlow(event.Flow)
		}
	}
}

func GetConntrackEvents() (chan conntrack.Event, chan error) {
	conn, err := conntrack.Dial(nil)
	if err != nil {
		log.Fatal(err)
	}

	eventChannel := make(chan conntrack.Event, 1024)
	errorChannel, err := conn.Listen(eventChannel, 4, netfilter.GroupsCT)
	if err != nil {
		log.Fatal(err)
	}

	err = conn.SetOption(netlink.ListenAllNSID, true)
	if err != nil {
		log.Fatal(err)
	}
	return eventChannel, errorChannel
}
