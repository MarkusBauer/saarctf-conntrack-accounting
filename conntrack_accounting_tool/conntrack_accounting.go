package main

// echo 1 > /proc/sys/net/netfilter/nf_conntrack_acct

import (
	"flag"
	"github.com/ti-mo/conntrack"
	"golang.org/x/sys/unix"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const NetfilterConntrackAcctSetting = "/proc/sys/net/netfilter/nf_conntrack_acct"

// 2020 we saw at most 117696 entries. That means: this pipe has a buffer for 285 bytes / entry.
const PipeBufferSize = 32 * 1024 * 1024

// Source filter configuration (from command line)
var SourceFilterPresent bool
var SourceFilterNet = net.IPNet{
	IP:   net.IPv4(0, 0, 0, 0),
	Mask: net.IPv4Mask(0, 0, 0, 0),
}
var SourceGroupMask = net.IPv4Mask(255, 255, 255, 255)

// Destination filter configuration (from command line)
var DestFilterPresent bool
var DestFilterNet = net.IPNet{
	IP:   net.IPv4(0, 0, 0, 0),
	Mask: net.IPv4Mask(0, 0, 0, 0),
}
var DestGroupMask = net.IPv4Mask(255, 255, 255, 255)

// Exclude this IP
var IpExcludePresent bool
var IpExclude net.IP

// Include ICMP?
var ICMPInclude bool

// Output file, default is stdout
var Output = os.Stdout

// Folder to store csv files
var OutputFolder string

// Interval to output summaries (in seconds)
var Interval int64 = 15

// Track open connections (and output them in every interval)
var TrackOpenConnections bool

// Check if we should consider a conntrack flow (after src / dst filter)
func FlowIsInteresting(flow *conntrack.Flow) bool {
	if flow.TupleOrig.IP.IsIPv6() || (flow.TupleOrig.Proto.Protocol == PROTO_ICMP && !ICMPInclude) {
		return false
	}
	if IpExcludePresent && (IpExclude.Equal(flow.TupleOrig.IP.SourceAddress) || IpExclude.Equal(flow.TupleOrig.IP.DestinationAddress)) {
		return false
	}
	if SourceFilterPresent && !SourceFilterNet.Contains(flow.TupleOrig.IP.SourceAddress) {
		return false
	}
	if DestFilterPresent && !DestFilterNet.Contains(flow.TupleOrig.IP.DestinationAddress) {
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

func handleAllChannels() {
	conntrackEventChannel, conntrackErrorChannel := GetConntrackEvents()
	signalChannel := WaitForTerminationChannel()
	dumpingChannel := GetDumpingChannel()
	portfileReloadChannel := PortFileReloadChannel()
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
				log.Fatal("Socket error channel (events):", err)
			}
			return
		case sig := <-signalChannel:
			log.Println("[Signal] Terminating with signal \"" + sig.String() + "\" ...")
			if TrackOpenConnections {
				accountOpenConnections()
			}
			FlushAccountingTableToOutput(time.Now())
			return
		case <-portfileReloadChannel:
			err := PortFileReload()
			if err != nil {
				log.Println("[Ports] Could not load file: ", err)
			}
		case dump := <-dumpingChannel:
			handleDump(dump)
			log.Println("[Events]", interestingEventCounter, "("+strconv.Itoa(eventCounter)+") events since last update")
			eventCounter = 0
			interestingEventCounter = 0
			if TrackOpenConnections {
				accountOpenConnections()
			}
			FlushAccountingTableToOutput(dump.Timestamp)
			go runDumping(dumpingChannel, nextTimestamp(Interval))
		}
	}
}

func main() {
	//IPv4 only for now
	srcfilter := flag.String("src", "", "Source network filter (CIDR notation)")
	srcfilterMask := flag.String("src-group-mask", "255.255.255.255", "Source filter mask")
	dstfilter := flag.String("dst", "", "Destination network filter (CIDR notation)")
	dstfilterMask := flag.String("dst-group-mask", "255.255.255.255", "Destination filter mask")
	excludeIP := flag.String("exclude-ip", "", "Exclude connections from or to a single IP")
	includeICMP := flag.Bool("include-icmp", false, "Include ICMP sessions")
	pipeFile := flag.String("pipe", "", "Pipe file to use")
	outputFolder := flag.String("output", "", "Output folder to store csv data")
	interval := flag.Int64("interval", 15, "Output interval")
	portFile := flag.String("ports", "", "File listing ports to track")
	flag.BoolVar(&TrackOpenConnections, "track-open", false, "Track open connections")
	flag.Parse()

	if srcfilter != nil && *srcfilter != "" {
		_, netrange, err := net.ParseCIDR(*srcfilter)
		if err != nil {
			log.Fatal("Invalid src filter:", err)
		}
		SourceFilterNet = *netrange
		SourceFilterPresent = true
		log.Printf("Source filter: %s\n", SourceFilterNet)
	}
	if dstfilter != nil && *dstfilter != "" {
		_, netrange, err := net.ParseCIDR(*dstfilter)
		if err != nil {
			log.Fatal("Invalid dst filter:", err)
		}
		DestFilterNet = *netrange
		DestFilterPresent = true
		log.Printf("Destination filter: %s\n", DestFilterNet)
	}
	SourceGroupMask = net.IPMask(net.ParseIP(*srcfilterMask).To4())
	DestGroupMask = net.IPMask(net.ParseIP(*dstfilterMask).To4())
	if excludeIP != nil && *excludeIP != "" {
		IpExclude = net.ParseIP(*excludeIP)
		IpExcludePresent = true
		log.Printf("Exclude IP: %s\n", IpExclude)
	}

	if includeICMP != nil {
		ICMPInclude = *includeICMP
	}

	if outputFolder != nil && *outputFolder != "" {
		OutputFolder = *outputFolder
		_ = os.Mkdir(OutputFolder, 0o755)
	}

	if pipeFile != nil && *pipeFile != "" {
		err := syscall.Mkfifo(*pipeFile, 0644)
		if err != nil && !os.IsExist(err) {
			log.Fatal(err)
		}
		isNewPipe := err == nil
		/*defer func() {
			err := os.Remove(*pipeFile)
			if err != nil {
				log.Println("Error removing pipe:", err)
			}
		}()*/

		Output, err = os.OpenFile(*pipeFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0777)
		if err != nil {
			log.Fatal(err)
		}
		defer func() {
			err := Output.Close()
			if err != nil {
				log.Println("Error closing output:", err)
			}
		}()
		// Set the size of the pipe's buffer
		if isNewPipe {
			_, err = unix.FcntlInt(Output.Fd(), unix.F_SETPIPE_SZ, PipeBufferSize)
			if err != nil {
				log.Println("Could not change pipe buffer size: ", err)
			}
			pipeBuffer, err := unix.FcntlInt(Output.Fd(), unix.F_GETPIPE_SZ, 0)
			if err != nil {
				log.Println("Could not determine pipe buffer size: ", err)
			} else {
				log.Println("Pipe buffer size: ", pipeBuffer)
			}
		}
		log.Println("Writing output to pipe \"" + *pipeFile + "\" ...")
	}

	if interval != nil && *interval > 1 {
		Interval = *interval
	}

	if portFile != nil && *portFile != "" {
		err := PortFileInit(*portFile)
		if err != nil {
			log.Fatal("Port file:", err)
		}
	}

	err := EnableNetfilterTrafficAccounting()
	if err != nil {
		log.Println("Could not check or enable conntrack traffic accounting. ")
		log.Println("Use: echo 1 > " + NetfilterConntrackAcctSetting)
	}
	handleAllChannels()
}
