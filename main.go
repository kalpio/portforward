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
	"time"

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

const VERSION = "2021.09.30b"

func configLog() {
	logFileName := fmt.Sprintf("%s.log", time.Now().Format("2006-01-02"))
	f, err := os.OpenFile(fmt.Sprintf("./logs/%s", logFileName), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}

	multiWriter := io.MultiWriter(f, os.Stdout)
	log.SetOutput(multiWriter)
}

func main() {
	flag.StringVar(&target, "target", "", "target (<host>:<port>)")
	flag.IntVar(&port, "port", 1337, "port")
	flag.StringVar(&iniFile, "inifile", "", "initialize file")
	flag.Parse()

	logInfo(fmt.Sprintf("Version: %s", VERSION))

	configLog()

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

var connID uint64

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

		connID++
		logInfo(fmt.Sprintf("[%d] CLIENT: %s", connID, infoConnAddresses(client)))
		logInfo(fmt.Sprintf("[%d] TARGET: %s", connID, target))
		logInfo(fmt.Sprintf("[%d] client %s connected!", connID, infoConnAddresses(client)))

		go handleRequest(connID, client, target)
	}
}

func handleRequest(requestID uint64, conn net.Conn, target string) {
	targetConn, err := net.Dial("tcp", target)
	if err != nil {
		logError(fmt.Sprintf("[%d] could not connect to target [%s]: %v", requestID, target, err))
		return
	}
	logInfo(fmt.Sprintf("[%d] connection to server %v established!", requestID, targetConn.RemoteAddr()))

	var wg sync.WaitGroup
	wg.Add(1)
	go copyIO(requestID, conn, targetConn, &wg)

	wg.Add(1)
	go copyIO(requestID, targetConn, conn, &wg)

	wg.Wait()

	if err := targetConn.Close(); err != nil {
		logError(fmt.Sprintf("[%d] could not close target %s: %v", requestID, infoConnAddresses(targetConn), err))
	}
}

func copyIO(requestID uint64, src, dst net.Conn, wg *sync.WaitGroup) {
	defer wg.Done()
	logInfo(fmt.Sprintf("[%d] starts copying %s => %s", requestID, infoConnAddresses(src), infoConnAddresses(dst)))
	written, err := io.Copy(dst, src)
	if err != nil {
		logError(fmt.Sprintf("[%d] copy data from: %q to %q: %v", requestID, infoConnAddresses(src), infoConnAddresses(dst), err))
		return
	}
	logInfo(fmt.Sprintf("[%d] copy data: %q to %q | %s", requestID, infoConnAddresses(src), infoConnAddresses(dst), humanize.Bytes(uint64(written))))
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

func infoConnAddresses(conn net.Conn) string {
	return fmt.Sprintf("[LOCAL: %s | REMOTE: %s]", conn.LocalAddr(), conn.RemoteAddr())
}
