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

const VERSION = "2021.09.23a"

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
			createListenerForPort(s.Port)
			go newListener(s.Target, s.Port, &wg)
		}
	} else {
		wg.Add(1)
		createListenerForPort(port)
		go newListener(target, port, &wg)
	}
	logInfo("waiting for request!")
	wg.Wait()
}

var listeners map[int]net.Listener

func createListenerForPort(port int) {
	if listeners == nil {
		listeners = make(map[int]net.Listener)
	}
	_, ok := listeners[port]
	if !ok {
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		listeners[port] = listener
		if err != nil {
			logError(err.Error())
		}
	}
}

func newListener(target string, port int, wg *sync.WaitGroup) {
	defer wg.Done()
	listener, ok := listeners[port]
	if !ok {
		logError(fmt.Sprintf("could not find listener for port %d", port))
	}

	for {
		client, err := listener.Accept()
		if err != nil {
			logError(fmt.Sprintf("could not accept client connection: %v", err))
			continue
		}

		logInfo(fmt.Sprintf("CLIENT: local addr: %s", client.LocalAddr()))
		logInfo(fmt.Sprintf("CLIENT: remote addr: %s target: %s", client.RemoteAddr(), target))
		logInfo(fmt.Sprintf("client %q connected!", client.RemoteAddr()))

		go handleRequest(client, target)
	}
}

func handleRequest(conn net.Conn, target string) {
	targetConn, err := net.Dial("tcp", target)
	if err != nil {
		logError(fmt.Sprintf("could not connect to target [%s]: %v", target, err))
		return
	}
	logInfo(fmt.Sprintf("connection to server %v established!", targetConn.RemoteAddr()))

	var wg sync.WaitGroup
	wg.Add(1)
	go copyIO(conn, targetConn, &wg)

	wg.Add(1)
	go copyIO(targetConn, conn, &wg)

	wg.Wait()

	if err := targetConn.Close(); err != nil {
		logError(fmt.Sprintf("could not close target %s: %v", targetConn.LocalAddr(), err))
	}
}

func copyIO(src, dst net.Conn, wg *sync.WaitGroup) {
	defer wg.Done()
	logInfo(fmt.Sprintf("starts copying %s => %s", src.LocalAddr(), dst.RemoteAddr()))
	written, err := io.Copy(dst, src)
	if err != nil {
		logError(fmt.Sprintf("copy data from: %q to %q: %v", src.LocalAddr(), dst.RemoteAddr(), err))
	}
	logInfo(fmt.Sprintf("copy data: %q to %q | %s", src.LocalAddr(), dst.RemoteAddr(), humanize.Bytes(uint64(written))))
}

func logError(str string) {
	log.Println(fmt.Sprintf("[ERROR] %s", str))
}

func logInfo(str string) {
	log.Println(fmt.Sprintf("[INFO] %s", str))
}

func logFatal(str string) {
	log.Fatalf(fmt.Sprintf("[FATAL]: %s", str))
}
