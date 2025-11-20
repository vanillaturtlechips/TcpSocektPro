package main

import (
	"bufio"
	"io"
	"log"
	"net"
	"strings"
	"time"
)

const (
	ListenPort     = ":9000"
	ReadTimeout    = 60 * time.Second
	WriteTimeout   = 5 * time.Second
	MaxBufferBytes = 4096
)

func main() {
	listener, err := net.Listen("tcp", ListenPort)
	if err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
	defer listener.Close()

	log.Printf("Server listening on %s", ListenPort)

	for {

		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			continue
		}

		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	remoteAddr := conn.RemoteAddr().String()
	log.Printf("Client connected: %s", remoteAddr)

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	for {
		err := conn.SetReadDeadline(time.Now().Add(ReadTimeout))
		if err != nil {
			log.Printf("SetReadDeadLine failed: %v", err)
			return
		}

		message, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				log.Printf("Read error from %s: %v", remoteAddr, err)
			}
			return
		}

		message = strings.TrimSpace(message)

		if message == "PING" {
			log.Printf("Heartbeat received from %s", remoteAddr)

			conn.SetWriteDeadline(time.Now().Add(WriteTimeout))
			_, err = writer.WriteString("PONG\n")
			if err != nil {
				return
			}
			writer.Flush()
			continue
		}

		log.Printf("Received from %s: %s", remoteAddr, message)

		conn.SetWriteDeadline(time.Now().Add(WriteTimeout))
		_, err = writer.WriteString("ECHO:" + message + "\n")
		if err != nil {
			log.Printf("Write error: %v", err)
			return
		}

		writer.Flush()
	}
}
