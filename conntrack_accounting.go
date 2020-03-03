package main

import (
	"flag"
	"fmt"
	"github.com/ti-mo/conntrack"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
)

// Direction:
// - Create a named pipe as output
// - Dump accounting information there
// - Close pipe afterwards

var SourceFilterPresent bool
var SourceFilterIP net.IP = net.IPv4(0, 0, 0, 0)
var SourceFilterMask net.IPMask = net.IPv4Mask(255, 255, 255, 255)
var DestFilterPresent bool
var DestFilterIP net.IP = net.IPv4(0, 0, 0, 0)
var DestFilterMask net.IPMask = net.IPv4Mask(255, 255, 255, 255)
var Output *os.File = os.Stdout

// Check if we should consider a conntrack flow (after src / dst filter)
func FlowIsInteresting(flow *conntrack.Flow) bool {
	if flow.TupleOrig.IP.IsIPv6() {
		return false
	}
	if SourceFilterPresent && !flow.TupleOrig.IP.SourceAddress.Mask(SourceFilterMask).Equal(SourceFilterIP) {
		return false
	}
	if DestFilterPresent && !flow.TupleOrig.IP.DestinationAddress.Mask(DestFilterMask).Equal(DestFilterIP) {
		return false
	}
	return true
}

// Create a channel that delivers termination signals
func WaitForTerminationChannel() chan os.Signal {
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	return signalChannel
}

func MainLoop() {
	conntrackEventChannel, conntrackErrorChannel := GetConntrackEvents()
	signalChannel := WaitForTerminationChannel()
	fmt.Println("Running ...")

	for {
		select {
		case event := <-conntrackEventChannel:
			if event.Flow != nil && FlowIsInteresting(event.Flow) {
				handleConntrackEvent(event)
			}
		case err := <-conntrackErrorChannel:
			if err != nil {
				log.Fatal("Socket error channel:", err)
			}
			return
		case sig := <-signalChannel:
			fmt.Println("Terminating with signal " + sig.String() + " ...")
			FlushAccountingTableToOutput()
			return
		}
	}
}

func main() {
	//IPv4 only for now
	srcfilter := flag.String("src", "", "Source filter")
	srcfilterMask := flag.String("srcmask", "255.255.255.255", "Source filter mask")
	dstfilter := flag.String("dst", "", "Destination filter")
	dstfilterMask := flag.String("dstmask", "255.255.255.255", "Destination filter mask")
	pipeFile := flag.String("pipe", "/tmp/conntrack_acct", "Pipe file to use")
	flag.Parse()

	if srcfilter != nil && *srcfilter != "" {
		if srcfilterMask != nil {
			SourceFilterMask = net.IPMask(net.ParseIP(*srcfilterMask).To4())
		}
		SourceFilterIP = net.ParseIP(*srcfilter).Mask(SourceFilterMask)
		SourceFilterPresent = true
		fmt.Printf("Source filter: %s/%s\n", SourceFilterIP, SourceFilterMask)
	}
	if dstfilter != nil && *dstfilter != "" {
		if dstfilterMask != nil {
			DestFilterMask = net.IPMask(net.ParseIP(*dstfilterMask).To4())
		}
		DestFilterIP = net.ParseIP(*dstfilter).Mask(DestFilterMask)
		DestFilterPresent = true
		fmt.Printf("Destination filter: %s/%s\n", DestFilterIP, DestFilterMask)
	}

	if pipeFile != nil && *pipeFile != "" {
		err := os.Remove(*pipeFile)
		if err != nil {
			log.Fatal(err)
		}
		err = syscall.Mkfifo(*pipeFile, 0660)
		defer os.Remove(*pipeFile)
		if err != nil {
			log.Fatal(err)
		}
		Output, err = os.OpenFile(*pipeFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0777)
		if err != nil {
			log.Fatal(err)
		}
		defer Output.Close()
	}

	MainLoop()
}
