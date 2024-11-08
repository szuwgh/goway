package core

import (
	"bufio"
	"log"
	"net"
	"net/http"
)

func Proxy() {
	lt, err := net.Listen("tcp", ":8082")
	if err != nil {
		log.Fatal(err)
	}

	defer lt.Close()

	for {
		conn, err := lt.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		go handleConn(conn)
	}
}

func handleConn(conn net.Conn) {
	req, err := http.ReadRequest(bufio.NewReader(conn))
	if err != nil {
		log.Println(err)
		return
	}
	log.Println(req.Host)
	log.Println(req.Method)
}
