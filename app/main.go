package main

import (
	"fmt"
	"net"
	"os"
	"strings"
)

func main() {
	// l is a net.Listener that listens on port 4221 on all interfaces.
	// You can use it to accept connections.
	// See https://pkg.go.dev/net#Listen for more information.
	l, err := net.Listen("tcp", "0.0.0.0:4221")
	if err != nil {
		fmt.Println("Failed to bind to port 4221")
		os.Exit(1)
	}
	defer l.Close()
	fmt.Println("Server is listening on 4221")

	// Accept a connection. This blocks until a connection is made.
	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting connection: ", err.Error())
			continue
		}
		handleClient(conn)
	}

}

func handleClient(conn net.Conn) {
	defer conn.Close()
	requestData := readRequest(conn)

	if requestData == "" {
		fmt.Println("Empty request data")
		return
	}

	url := parseUrl(requestData)
	if strings.Contains(url, "echo") {
		content := strings.Split(url, "/")[2]
		resp := fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s",
			len(content), content)
		_, err := conn.Write([]byte(resp))
		if err != nil {
			fmt.Println("Error writing to connection: ", err.Error())
		}
	} else if url == "/" {
		_, err := conn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))
		if err != nil {
			fmt.Println("Error writing to connection: ", err.Error())
		}
	} else {
		_, err := conn.Write([]byte("HTTP/1.1 404 Not Found\r\n\r\n"))
		if err != nil {
			fmt.Println("Error writing to connection: ", err.Error())
		}
	}
}

func parseUrl(s string) string {
	request := strings.Split(s, "\r\n")
	requestLine := request[0]
	return strings.Split(requestLine, " ")[1]
}

func readRequest(conn net.Conn) string {
	buf := make([]byte, 1024)
	isMoredata := true
	var sb strings.Builder

	for isMoredata {
		numberBytes, err := conn.Read(buf)
		if err != nil {
			fmt.Println("Error reading buffer: ", err.Error())
			return ""
		}
		if numberBytes < 1024 {
			isMoredata = false
		}
		sb.Write(buf[:numberBytes])
	}
	return sb.String()
}
