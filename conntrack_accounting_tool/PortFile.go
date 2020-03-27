package main

import (
	"bufio"
	"errors"
	"github.com/fsnotify/fsnotify"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
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
var reloadChannel chan bool = make(chan bool, 1)

func checkReloads() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal("fsnotify.NewWatcher:", err)
	}
	defer watcher.Close()

	err = watcher.Add(portfile)
	if err != nil {
		log.Fatal("watcher.Add: ", err)
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			// log.Println("event:", event)
			if event.Op&fsnotify.Write == fsnotify.Write {
				// log.Println("modified file:", event.Name)
				time.Sleep(time.Duration(250000000)) // 250ms delay
				reloadChannel <- true
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				log.Println("fswatcher fatal error:", err)
				return
			}
			log.Println("fswatcher error:", err)
		}
	}
}

func PortFileReloadChannel() chan bool {
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
