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

func main() {
	flag.StringVar(&target, "target", "", "target (<host>:<port>)")
	flag.IntVar(&port, "port", 1337, "port")
	flag.StringVar(&iniFile, "inifile", "", "initialize file")
	flag.Parse()

	var wg sync.WaitGroup
	if len(iniFile) > 0 {
		var settings []iniFileSetting
		f, err := os.Open(iniFile)
		if err != nil {
			log.Fatalf("could not open file: %s: %v", iniFile, err)
		}
		bytes, err := ioutil.ReadAll(f)
		if err != nil {
			log.Fatalf("could not read data from stream: %v", err)
		}
		if err := json.Unmarshal(bytes, &settings); err != nil {
			log.Fatalf("could not unmarshal data: %v", err)
		}

		for _, s := range settings {
			wg.Add(1)
			go newListener(s.Target, s.Port, &wg)
		}

	} else {
		wg.Add(1)
		go newListener(target, port, &wg)
	}
	wg.Wait()
}

func newListener(target string, port int, wg *sync.WaitGroup) {
	defer wg.Done()
	incoming, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("could not start server on %d: %v", port, err)
	}
	log.Printf("address: %v\n", incoming.Addr())
	log.Printf("server running on %d\n", port)

	for {
		client, err := incoming.Accept()
		if err != nil {
			log.Fatalf("could not accept client connection: %v", err)
		}
		defer func() {
			remoteAddr := client.RemoteAddr()
			log.Printf("closing client %s...\n", remoteAddr)
			client.Close()
			log.Printf("client %s closed!\n", remoteAddr)
		}()
		log.Printf("client %q connected!\n", client.RemoteAddr())

		go handleRequest(client, target)
	}
}

func handleRequest(conn net.Conn, target string) {
	targetConn, err := net.Dial("tcp", target)
	if err != nil {
		log.Fatalf("could not connect to target: %v", err)
	}
	// defer target.Close()
	log.Printf("connection to server %v established!\n", targetConn.RemoteAddr())

	go copyIO(conn, targetConn)
	go copyIO(targetConn, conn)

	// go func() { io.Copy(target, conn) }()
	// go func() { io.Copy(conn, target) }()
}

func copyIO(src, dst net.Conn) {
	written, err := io.Copy(dst, src)
	if err != nil {
		log.Printf("ERROR: could not copy data from: [%q] to [%q]: %v\n", src.LocalAddr(), dst.RemoteAddr(), err)
	}
	log.Printf("[%q] => [%q] | %s\n", src.LocalAddr(), dst.RemoteAddr(), humanize.Bytes(uint64(written)))
}
