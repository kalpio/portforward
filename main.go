package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"sync"

	"github.com/dustin/go-humanize"
)

var (
	target  string
	port    int
	iniFile string
)

type iniFileSetting struct {
	Target string `json:"target"`
	Port   int    `json:"port"`
}

const VERSION = "2021.08.10"

func main() {
	flag.StringVar(&target, "target", "", "target (<host>:<port>)")
	flag.IntVar(&port, "port", 1337, "port")
	flag.StringVar(&iniFile, "inifile", "", "initialize file")
	flag.Parse()

	logInfo(fmt.Sprintf("Version: %s", VERSION))

	var wg sync.WaitGroup
	if len(iniFile) > 0 {
		var settings []iniFileSetting
		f, err := os.Open(iniFile)
		if err != nil {
			logFatal(fmt.Sprintf("could not open initialize file: %s: %v", iniFile, err))
		}
		bytes, err := ioutil.ReadAll(f)
		if err != nil {
			logFatal(fmt.Sprintf("could not read data from stream: %v", err))
		}
		if err := json.Unmarshal(bytes, &settings); err != nil {
			logFatal(fmt.Sprintf("could not unmarshall data: %v", err))
		}

		for _, s := range settings {
			wg.Add(1)
			go newListener(s.Target, s.Port, &wg)
		}
	} else {
		wg.Add(1)
		go newListener(target, port, &wg)
	}
	logInfo("waiting for request!")
	wg.Wait()
}

func newListener(target string, port int, wg *sync.WaitGroup) {
	defer wg.Done()
	incoming, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		logError(fmt.Sprintf("could not start server on %d: %v", port, err))
	}
	logInfo(fmt.Sprintf("for destination %s server running on %s", target, incoming.Addr()))

	for {
		client, err := incoming.Accept()
		if err != nil {
			logError(fmt.Sprintf("could not accept client connection: %v", err))
		}
		defer func() {
			remoteAddr := client.RemoteAddr()
			logInfo(fmt.Sprintf("closing client %s...", remoteAddr))
			if err := client.Close(); err != nil {
				logError(fmt.Sprintf("closing client %s: %v", remoteAddr, err))
			}
			logInfo(fmt.Sprintf("client %s closed!", remoteAddr))
		}()
		logInfo(fmt.Sprintf("client %q connected!", client.RemoteAddr()))

		go handleRequest(client, target)
	}
}

func handleRequest(conn net.Conn, target string) {
	targetConn, err := net.Dial("tcp", target)
	if err != nil {
		logError(fmt.Sprintf("could not connect to target: %v", err))
		return
	}
	logInfo(fmt.Sprintf("connection to server %v established!", targetConn.RemoteAddr()))

	go copyIO(conn, targetConn)
	go copyIO(targetConn, conn)
}

func copyIO(src, dst net.Conn) {
	written, err := io.Copy(dst, src)
	if err != nil {
		logError(fmt.Sprintf("copy data from: %q to %q: %v", src.LocalAddr(), dst.RemoteAddr(), err))
	}
	logInfo(fmt.Sprintf("copy data: %q to %q | %s", src.LocalAddr(), dst.RemoteAddr(), humanize.Bytes(uint64(written))))
}

func logError(str string) {
	log.Printf(fmt.Sprintln(fmt.Sprintf("[ERROR] %s", str)))
}

func logInfo(str string) {
	log.Printf(fmt.Sprintln(fmt.Sprintf("[INFO] %s", str)))
}

func logFatal(str string) {
	log.Fatalf(fmt.Sprintf("[FATAL]: %s", str))
}
