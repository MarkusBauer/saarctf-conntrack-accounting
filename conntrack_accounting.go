package conntrack_accounting

import (
	"fmt"
	"github.com/mdlayher/netlink"
	"github.com/ti-mo/conntrack"
	"github.com/ti-mo/netfilter"
	"log"
)

// Direction:
// - Create a named pipe as output
// - Dump accounting information there
// - Close pipe afterwards

func flowIsInteresting(flow *conntrack.Flow) bool {
	//TODO
	return false
}

func handleNewFlow(flow *conntrack.Flow) {
	//TODO
}

func handleDestroyFlow(flow *conntrack.Flow) {
	//TODO
}

func handleTerminateFlow(flow *conntrack.Flow) {
	//TODO
}

func ListenForConntrackEvents() {
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

	for {
		select {
		case event := <-eventChannel:
			if event.Flow == nil || !flowIsInteresting(event.Flow) {
				continue
			}

			switch event.Type {
			case conntrack.EventNew:
				handleNewFlow(event.Flow)
			case conntrack.EventDestroy:
				handleDestroyFlow(event.Flow)
			case conntrack.EventUpdate:
				// Check if we know this flow and should terminate it
				state := event.Flow.ProtoInfo.TCP.State
				if state == TCP_CONNTRACK_CLOSE_WAIT || state == TCP_CONNTRACK_LAST_ACK || state == TCP_CONNTRACK_CLOSE {
					if _, exists := connections[event.Flow.ID]; exists {
						handleTerminateFlow(event.Flow)
					}
				}
			}
		case err := <-errorChannel:
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println("Terminating...")
			return
		}
	}
}

func main() {
	// TODO some filter options
	go ListenForConntrackEvents()
}
