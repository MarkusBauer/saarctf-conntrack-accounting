package main

import (
	"flag"
	"github.com/fsnotify/fsnotify"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func watchFolderForCSV(directory string) chan string {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal("fsnotify.NewWatcher:", err)
	}

	err = watcher.Add(directory)
	if err != nil {
		log.Fatal("watcher.Add: ", err)
	}

	files := make(chan string, 512)

	go func() {
		defer watcher.Close()
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					log.Fatal("fswatcher event not ok", event)
				}
				// log.Println("event:", event)
				if event.Op&fsnotify.Write == fsnotify.Write {
					// log.Println("modified file:", event.Name)
					//time.Sleep(time.Duration(200000000)) // 200ms delay
					if strings.HasSuffix(strings.ToLower(event.Name), ".csv") {
						files <- event.Name
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					log.Fatal("fswatcher fatal error:", err)
				}
				log.Println("fswatcher error:", err)
			}
		}
	}()

	return files
}

func deduplicateEvents(input chan string) chan string {
	output := make(chan string, 256)
	go func() {
		old_fname := ""
		for {
			timer := time.NewTimer(1500 * time.Millisecond)
			select {
			case fname := <-input:
				if old_fname != fname && old_fname != "" {
					output <- old_fname
				}
				old_fname = fname
			case <-timer.C:
				if old_fname != "" {
					output <- old_fname
				}
				old_fname = ""
			}
			timer.Stop()
		}
	}()
	return output
}

// Create a channel that delivers termination signals
func WaitForTerminationChannel() chan os.Signal {
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	return signalChannel
}

func main() {
	hostname := flag.String("host", "localhost", "Postgresql hostname")
	database := flag.String("db", "", "Postgresql database")
	username := flag.String("user", "", "Postgresql username")
	passwd := flag.String("pass", "", "Postgresql password")
	watchFolder := flag.String("watch", "", "Watch this folder for incoming csv's")
	flag.Parse()

	db := Database{}
	err := db.Open(*username, *passwd, *hostname, *database)
	if err != nil {
		log.Fatal("DB open:", err)
	}
	defer db.Close()

	// create table
	db.CreateTable()

	for _, fname := range flag.Args() {
		db.InsertCSV(fname)
	}

	if watchFolder != nil && *watchFolder != "" {
		files := watchFolderForCSV(*watchFolder)
		files = deduplicateEvents(files)
		signalChannel := WaitForTerminationChannel()
		for {
			select {
			case fname := <-files:
				log.Printf("Loading file %s ...\n", fname)
				db.InsertCSV(fname)
			case sig := <-signalChannel:
				log.Println("[Signal] Terminating with signal \"" + sig.String() + "\" ...")
				return
			}
		}
	}
}
