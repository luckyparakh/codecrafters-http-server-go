package main

import (
	"bufio"
	"fmt"
	"net"
)

type Response struct {
	StatusCode int
	StatusText string
	Headers    map[string]string
	Body       []byte
}

func NewResponse(statusCode int, statusText string, body []byte) *Response {
	return &Response{
		StatusCode: statusCode,
		StatusText: statusText,
		Body:       body,
	}
}

func (r *Response) SetHeader(key, value string) {
	r.Headers[key] = value
}

func writeResponse(conn net.Conn, resp *Response) error {
	/*
	   WHY bufio.Writer instead of strings.Builder?

	   strings.Builder approach:
	     var sb strings.Builder
	     sb.WriteString("headers...")      // Build in memory
	     sb.Write(bodyBytes)               // Build in memory
	     conn.Write([]byte(sb.String()))   // Convert & send (1 syscall)

	     Issues:
	       - TWO buffers: strings.Builder + kernel network buffer
	       - Holds ENTIRE response in memory before sending
	       - Cannot stream/send partial data
	       - Extra memory allocation for full response

	   bufio.Writer approach (BETTER for network I/O):
	     w := bufio.NewWriter(conn)        // One buffer directly to network
	     w.WriteString("headers...")       // Buffered (no syscall yet)
	     w.Write(bodyBytes)                // Buffered (no syscall yet)
	     w.Flush()                         // Send all buffered data (1 syscall)

	     Advantages:
	       ✓ ONE buffer: directly to network (no intermediate string buffer)
	       ✓ Batches multiple small writes into one syscall
	       ✓ Can flush partially (streaming capable)
	       ✓ More memory efficient (no duplicate buffers)
	       ✓ Standard Go idiom for network I/O

	     Note: w.WriteString() does convert string → []byte internally,
	           BUT it writes directly to the network buffer, not an
	           intermediate string buffer like strings.Builder

	   Real benefit: Fewer memory allocations + fewer syscalls + streaming support

	   Conclusion: bufio.Writer is the Go idiom for network I/O
	               (Used internally by net/http standard library)
	*/
	w := bufio.NewWriter(conn)

	// Write status line
	statusLine := fmt.Sprintf("HTTP/1.1 %d %s\r\n", resp.StatusCode, resp.StatusText)
	if _, err := w.WriteString(statusLine); err != nil {
		return err
	}

	// If body is present, set Content-Length header, if not already set
	if len(resp.Body) > 0 {
		if _, exists := resp.Headers["Content-Length"]; !exists {
			resp.Headers["Content-Length"] = fmt.Sprintf("%d", len(resp.Body))
		}
	}

	// Write headers
	for key, value := range resp.Headers {
		if _, err := w.WriteString(fmt.Sprintf("%s: %s\r\n", key, value)); err != nil {
			return err
		}
	}

	// End of headers
	if _, err := w.WriteString("\r\n"); err != nil {
		return err
	}

	// Write body
	if len(resp.Body) > 0 {
		if _, err := w.Write(resp.Body); err != nil {
			return err
		}
	}

	return w.Flush()
}
