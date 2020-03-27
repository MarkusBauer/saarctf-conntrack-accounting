package main

import (
	"bufio"
	"errors"
	"log"
	"os"
	"strconv"
	"strings"
)

var interestingPorts = make(map[string]map[uint16]bool)

func PortIsInteresting(proto string, port uint16) bool {
	if len(interestingPorts) == 0 {
		return true
	}
	portsForProto := interestingPorts[proto]
	return portsForProto != nil && portsForProto[port]
}

var portfile string
var reloadChannel chan int = make(chan int, 1)

func checkReloads() {
	// TODO
}

func PortFileReloadChannel() chan int {
	return reloadChannel
}

func PortFileReload() error {
	file, err := os.Open(portfile)
	if err != nil {
		return err
	}
	defer file.Close()

	newInterestingPorts := make(map[string]map[uint16]bool)
	numEntries := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		protoport := strings.Split(line, ":")
		if len(protoport) != 2 {
			return errors.New("Invalid line (not format \"proto:port\"): " + line)
		}
		port, err := strconv.ParseUint(protoport[1], 10, 16)
		if err != nil {
			return err
		}

		if newInterestingPorts[protoport[0]] == nil {
			newInterestingPorts[protoport[0]] = make(map[uint16]bool)
		}
		newInterestingPorts[protoport[0]][uint16(port)] = true
		numEntries++
	}
	err = scanner.Err()
	if err == nil {
		interestingPorts = newInterestingPorts
		log.Printf("[Ports] Reload portfile with %d entries\n", numEntries)
	}
	return err
}

func PortFileInit(fname string) error {
	portfile = fname
	err := PortFileReload()
	if err == nil {
		go checkReloads()
	}
	return err
}
