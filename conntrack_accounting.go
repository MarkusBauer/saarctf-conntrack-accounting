package main

// echo 1 > /proc/sys/net/netfilter/nf_conntrack_acct

import (
	"flag"
	"github.com/ti-mo/conntrack"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
)

const NetfilterConntrackAcctSetting = "/proc/sys/net/netfilter/nf_conntrack_acct"

var SourceFilterPresent bool
var SourceFilterIP net.IP = net.IPv4(0, 0, 0, 0)
var SourceFilterMask net.IPMask = net.IPv4Mask(255, 255, 255, 255)
var DestFilterPresent bool
var DestFilterIP net.IP = net.IPv4(0, 0, 0, 0)
var DestFilterMask net.IPMask = net.IPv4Mask(255, 255, 255, 255)
var Output *os.File = os.Stdout
var Interval int64 = 15
var TrackOpenConnections bool

// Check if we should consider a conntrack flow (after src / dst filter)
func FlowIsInteresting(flow *conntrack.Flow) bool {
	if flow.TupleOrig.IP.IsIPv6() || flow.TupleOrig.Proto.Protocol == PROTO_ICMP {
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

func EnableNetfilterTrafficAccounting() error {
	content, err := ioutil.ReadFile(NetfilterConntrackAcctSetting)
	if err != nil {
		return err
	}
	if strings.Trim(string(content), " \n") == "0" {
		err = ioutil.WriteFile(NetfilterConntrackAcctSetting, []byte("1"), 0644)
		if err != nil {
			return err
		}
		log.Println("Enabled conntrack traffic accounting (" + NetfilterConntrackAcctSetting + " = 1)")
		log.Println("Connections that are already open cannot be tracked.")
	} else {
		log.Println("Conntrack traffic accounting is already enabled.")
	}
	return nil
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
	dumpingChannel := GetDumpingChannel()
	log.Println("Running ...")
	var eventCounter int
	var interestingEventCounter int

	for {
		select {
		case event := <-conntrackEventChannel:
			eventCounter++
			if event.Flow != nil && FlowIsInteresting(event.Flow) {
				interestingEventCounter++
				handleConntrackEvent(event)
			}
		case err := <-conntrackErrorChannel:
			if err != nil {
				log.Fatal("Socket error channel:", err)
			}
			return
		case sig := <-signalChannel:
			log.Println("[Signal] Terminating with signal \"" + sig.String() + "\" ...")
			if TrackOpenConnections {
				accountOpenConnections()
			}
			FlushAccountingTableToOutput()
			return
		case dump := <-dumpingChannel:
			handleDump(dump)
			log.Println("[Events]", interestingEventCounter, "("+strconv.Itoa(eventCounter)+") events since last update")
			eventCounter = 0
			interestingEventCounter = 0
			if TrackOpenConnections {
				accountOpenConnections()
			}
			FlushAccountingTableToOutput()
			go runDumping(dumpingChannel, nextTimestamp(Interval))
		}
	}
}

func main() {
	//IPv4 only for now
	srcfilter := flag.String("src", "", "Source filter")
	srcfilterMask := flag.String("srcmask", "255.255.255.255", "Source filter mask")
	dstfilter := flag.String("dst", "", "Destination filter")
	dstfilterMask := flag.String("dstmask", "255.255.255.255", "Destination filter mask")
	pipeFile := flag.String("pipe", "", "Pipe file to use")
	interval := flag.Int64("interval", 15, "Output interval")
	flag.BoolVar(&TrackOpenConnections, "track-open", false, "Track open connections")
	flag.Parse()

	if srcfilter != nil && *srcfilter != "" {
		if srcfilterMask != nil {
			SourceFilterMask = net.IPMask(net.ParseIP(*srcfilterMask).To4())
		}
		SourceFilterIP = net.ParseIP(*srcfilter).Mask(SourceFilterMask)
		SourceFilterPresent = true
		log.Printf("Source filter: %s/%s\n", SourceFilterIP, SourceFilterMask)
	}
	if dstfilter != nil && *dstfilter != "" {
		if dstfilterMask != nil {
			DestFilterMask = net.IPMask(net.ParseIP(*dstfilterMask).To4())
		}
		DestFilterIP = net.ParseIP(*dstfilter).Mask(DestFilterMask)
		DestFilterPresent = true
		log.Printf("Destination filter: %s/%s\n", DestFilterIP, DestFilterMask)
	}

	if pipeFile != nil && *pipeFile != "" {
		err := os.Remove(*pipeFile)
		if err != nil && !os.IsNotExist(err) {
			log.Fatal(err)
		}
		err = syscall.Mkfifo(*pipeFile, 0644)
		defer os.Remove(*pipeFile)
		if err != nil {
			log.Fatal(err)
		}
		Output, err = os.OpenFile(*pipeFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0777)
		if err != nil {
			log.Fatal(err)
		}
		defer Output.Close()
		log.Println("Writing output to pipe \"" + *pipeFile + "\" ...")
	}

	if interval != nil && *interval > 1 {
		Interval = *interval
	}

	err := EnableNetfilterTrafficAccounting()
	if err != nil {
		log.Println("Could not check or enable conntrack traffic accounting. ")
		log.Println("Use: echo 1 > " + NetfilterConntrackAcctSetting)
	}
	MainLoop()
}
