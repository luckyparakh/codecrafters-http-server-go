package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type Config struct {
	Port         string
	Host         string
	Protocol     string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}
type Server struct {
	listener net.Listener
	config   Config
	logger   *log.Logger
	wg       sync.WaitGroup
	router   *Router
}

var dirPath string

func main() {
	if len(os.Args) > 2 && os.Args[1] == "--directory" {
		dirPath = os.Args[2]
		if info, err := os.Stat(dirPath); err != nil || !info.IsDir() {
			log.Fatalf("Invalid directory: %s", dirPath)
		}
	}

	config := Config{
		Port:         "4221",
		Host:         "0.0.0.0",
		Protocol:     "tcp",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	logger := log.New(os.Stdout, "[http-server]", log.LstdFlags|log.Llongfile)
	server, err := NewServer(config, logger)
	if err != nil {
		logger.Fatalf("Failed to create server: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Println("Shutdown signal received, gracefully stopping.")
		cancel()
		server.Shutdown()
	}()

	if err := server.Start(ctx); err != nil {
		logger.Fatalf("Error starting server: %v", err)
	}
}

func NewServer(config Config, logger *log.Logger) (*Server, error) {
	addr := net.JoinHostPort(config.Host, config.Port)
	l, lErr := net.Listen(config.Protocol, addr)
	if lErr != nil {
		return nil, lErr
	}

	server := Server{
		listener: l,
		config:   config,
		logger:   logger,
		router:   NewRouter(),
	}

	server.RegisterRoutes()

	return &server, nil
}

func (s *Server) RegisterRoutes() {
	s.router.RegisterExactRoute("/", handleRoot)
	s.router.RegisterPrefixRoute(echoPrefix, handleEcho)
	s.router.RegisterExactRoute(userAgentPrefix, handleUserAgent)
	s.router.RegisterPrefixRoute(filesPrefix, handleFiles)
}

func (s *Server) Start(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		// Blocks until NEW connection (NOT New request on existing connection)
		conn, connErr := s.listener.Accept()
		if connErr != nil {
			// s.logger.Printf("failed to accept connection: %v", connErr)
			// continue
			select {
			case <-ctx.Done():
				return nil
			default:
				s.logger.Printf("Error accepting connection: %v", connErr)
				continue
			}
		}
		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

func (s *Server) Shutdown() {
	s.logger.Println("Shutdown Initiated")

	if err := s.listener.Close(); err != nil {
		s.logger.Printf("Error closing listener: %v", err)
	}

	s.logger.Println("Waiting for connection to finish")
	s.wg.Wait()
	s.logger.Println("Server Stopped")
}

func (s *Server) handleConnection(conn net.Conn) {
	defer s.wg.Done()
	defer func() {
		s.logger.Printf("Closing connection from %s", conn.RemoteAddr().String())
		if err := conn.Close(); err != nil {
			s.logger.Printf("Error closing connection: %v", err)
		}
	}()

	/*
		   HTTP/1.1 Keep-Alive (Persistent Connections):

		   By default, HTTP/1.1 keeps connections open for multiple requests.
		   Benefits:
		     - Reduces TCP handshake overhead
		     - Faster subsequent requests
		     - Less server resource usage

			Without loop (one request per connection):
				Client → Server: Open TCP, GET /a, Close TCP
				Client → Server: Open TCP, GET /b, Close TCP
				Cost: 2 TCP handshakes (~100ms overhead)

			With loop (multiple requests per connection):
				Client → Server: Open TCP
								GET /a → Response
								GET /b → Response (same connection!)
								Close TCP
				Cost: 1 TCP handshake (~50ms overhead)


		   We loop to handle multiple requests on the same connection until:
		     1. Client sends "Connection: close" header
		     2. Read timeout occurs (no more data)
		     3. Parse error (malformed request)
		     4. Client closes connection
	*/

	/*

		TCP Connection = Phone call between curl and server

		curl: [Dials] → Server: [Answers]
		      Server: Accept() returns, creates goroutine

		curl: "Hey, GET /a please"
		      [Speaks into phone]

		Server goroutine: [Listens on same phone]
		                  "Here's /a"
		                  [Still holding phone, waiting...]

		curl: "Great! Now GET /b please"
		      [Same phone call, no hang-up!]

		Server goroutine: [Still listening on same phone]
		                  "Here's /b"
		                  [Still holding phone...]

		curl: [Hangs up phone]

		Server goroutine: [Hears dial tone / EOF]
		                  [Hangs up]
	*/

	for {
		setReadDeadlineErr := conn.SetReadDeadline(time.Now().Add(s.config.ReadTimeout))
		if setReadDeadlineErr != nil {
			s.logger.Printf("Error setting read deadline: %v", setReadDeadlineErr)
			return
		}
		setWriteDeadlineErr := conn.SetWriteDeadline(time.Now().Add(s.config.WriteTimeout))
		if setWriteDeadlineErr != nil {
			s.logger.Printf("Error setting write deadline: %v", setWriteDeadlineErr)
			return
		}

		req, parseErr := parseRequest(conn)
		if parseErr != nil {
			if errors.Is(parseErr, io.EOF) {
				s.logger.Println("Client closed connection")
			} else {
				s.logger.Printf("Error parsing request: %v", parseErr)
			}
			return
		}
		s.logger.Printf("Received request: %+v", req)

		handler := s.router.Match(req.Path)
		resp := handler(req)
		s.logger.Printf("Response of the request: %+v", resp)

		if err := processCommonHeaders(req, resp); err != nil {
			s.logger.Printf("Error processing common headers: %v", err)
			return
		}
		s.logger.Printf("Response of the request after processing common headers: %+v", resp)

		if err := writeResponse(conn, resp); err != nil {
			s.logger.Printf("Error writing response: %v", err)
		}

		if val, ok := req.GetHeader("Connection"); ok && val == "close" {
			s.logger.Println("Connection: close header found, closing connection.")
			return
		}
	}
}

// Old Code

// func handleClient(conn net.Conn) {
// 	defer conn.Close()
// 	requestData := readRequest(conn)

// 	if requestData == "" {
// 		fmt.Println("Empty request data")
// 		return
// 	}

// 	url := parseUrl(requestData)
// 	if strings.Contains(url, "echo") {
// 		content := strings.Split(url, "/")[2]
// 		resp := fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s",
// 			len(content), content)
// 		_, err := conn.Write([]byte(resp))
// 		if err != nil {
// 			fmt.Println("Error writing to connection: ", err.Error())
// 		}
// 	} else if url == "/" {
// 		_, err := conn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))
// 		if err != nil {
// 			fmt.Println("Error writing to connection: ", err.Error())
// 		}
// 	} else {
// 		_, err := conn.Write([]byte("HTTP/1.1 404 Not Found\r\n\r\n"))
// 		if err != nil {
// 			fmt.Println("Error writing to connection: ", err.Error())
// 		}
// 	}
// }

// func parseUrl(s string) string {
// 	request := strings.Split(s, "\r\n")
// 	requestLine := request[0]
// 	return strings.Split(requestLine, " ")[1]
// }

// func readRequest(conn net.Conn) string {
// 	buf := make([]byte, 1024)
// 	isMoredata := true
// 	var sb strings.Builder

// 	for isMoredata {
// 		numberBytes, err := conn.Read(buf)
// 		if err != nil {
// 			fmt.Println("Error reading buffer: ", err.Error())
// 			return ""
// 		}
// 		if numberBytes < 1024 {
// 			isMoredata = false
// 		}
// 		sb.Write(buf[:numberBytes])
// 	}
// 	return sb.String()
// }
