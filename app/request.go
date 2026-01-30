package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
)

type Request struct {
	Method  string
	Path    string
	Version string
	Headers map[string]string
	Body    []byte
}

func parseRequest(conn net.Conn) (*Request, error) {
	reader := bufio.NewReader(conn)

	requestLine, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}

	// Raw data "GET /submit HTTP/1.1\r\n Host: localhost\r\n Content-Length: 13\r\n \r\n Hello, World!"

	// 1. Read request line
	// Example: GET /submit HTTP/1.1\r\n
	requestLine = strings.TrimSpace(requestLine)
	parts := strings.Fields(requestLine)

	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid request line: %s", requestLine)
	}
	req := &Request{
		Method:  parts[0],
		Path:    parts[1],
		Version: parts[2],
		Headers: make(map[string]string),
	}

	// 2. Read headers
	// Example: Host: localhost\r\n Content-Length: 13\r\n \r\n
	for {
		line, err := reader.ReadString('\n')

		if err != nil {
			return nil, err
		}

		line = strings.TrimSpace(line)
		// HTTP protocol specification: Headers are separated from the body by an empty line
		/*
			GET /echo/hello HTTP/1.1\r\n          ← Request line
			Host: localhost:4221\r\n               ← Header 1
			User-Agent: curl/7.0\r\n               ← Header 2
			\r\n                                    ← EMPTY LINE (signals end of headers)
			[optional body data]                   ← Body starts here
		*/
		if line == "" {
			break // End of headers
		}

		// Parse header line
		if strings.Contains(line, ":") {
			colonIdx := strings.Index(line, ":")
			key := strings.TrimSpace(line[:colonIdx])
			value := strings.TrimSpace(line[colonIdx+1:])
			req.Headers[strings.ToLower(key)] = value // Store headers in lowercase for case-insensitive access
		}
	}

	// Read body if Content-Length header is present
	contentLength, exists := req.Headers["Content-Length"]
	if exists {
		// 3. At this point, reader cursor is positioned at "Hello, World!"
		//    It has NOT re-read any previous data
		var length int

		// strconv.Atoi() converts string "123" to int 123
		// Returns error for invalid inputs like "abc", or empty string
		length, err := strconv.Atoi(contentLength)
		if err != nil {
			return nil, fmt.Errorf("invalid Content-Length: %w", err)
		}

		// Validate Content-Length
		if length < 0 {
			return nil, fmt.Errorf("negative Content-Length: %d", length)
		}

		// Prevent excessively large bodies
		if length > 10*1024*1024 { // 10 MB limit
			return nil, fmt.Errorf("Content-Length too large: %d", length)
		}

		if length > 0 {
			req.Body = make([]byte, length)
			/*
				WHY io.ReadFull() instead of reader.Read()?

				reader.Read() contract: "I'll read AT LEAST 1 byte, UP TO len(buf) bytes"
				  - Might return 10 bytes when you wanted 1000
				  - Network packets can arrive in chunks
				  - Example: 1000-byte body might arrive as:
				      First call:  300 bytes (rest are zeros!)
				      Need to call Read() again to get remaining 700 bytes

				io.ReadFull() contract: "I'll read EXACTLY len(buf) bytes or return error"
				  - Keeps reading internally until buffer is completely filled
				  - Handles TCP packet fragmentation automatically
				  - Guarantees: n == len(buf) OR error != nil

				Real scenario:
				  POST request with 20-byte body arriving in 2 TCP packets:
				  Packet 1: "1234567890" (10 bytes)
				  Packet 2: "1234567890" (10 bytes)

				  reader.Read():  returns 10, need manual retry
				  io.ReadFull():  waits and returns all 20 bytes ✓

				bufio.Reader maintains position - already consumed headers, now positioned at body start
			*/
			_, err := io.ReadFull(reader, req.Body)
			if err != nil {
				return nil, err
			}
		}
	}
	return req, nil
}
