package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"net"
	"strings"
)

var supportedCompression = map[string]bool{
	"gzip": true,
}

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
		Headers:    make(map[string]string),
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

func processCommonHeaders(r *Request, resp *Response) error {
	// Handle Accept-Encoding for compression
	if compressType, ok := r.GetHeader("Accept-Encoding"); ok {
		if err := compressBody(resp, compressType); err != nil {
			return err
		}
	}

	// If body is present, set Content-Length header, if not already set
	// This is important after compression, as body length may have changed
	if len(resp.Body) > 0 {
		if _, exists := resp.Headers["Content-Length"]; !exists {
			resp.Headers["Content-Length"] = fmt.Sprintf("%d", len(resp.Body))
		}
	}

	// Handle Connection: close
	if val, ok := r.GetHeader("Connection"); ok && val == "close" {
		resp.SetHeader("Connection", "close")
	}

	return nil
}

func compressBody(resp *Response, compressType string) error {
	// "Accept-Encoding: invalid-encoding-1, gzip, invalid-encoding-2"
	// Check each encoding in order, use the first supported one
	compressTypePart := strings.SplitSeq(strings.TrimSpace(compressType), ",")

	for ct := range compressTypePart {
		// Trim spaces around each encoding type
		ct = strings.TrimSpace(ct)

		// Check if this compression type is supported
		if supportedCompression[ct] {
			fmt.Println("compressType", ct)
			if err := doCompression(resp, ct); err != nil {
				// Log error but continue - try next encoding
				// May be next compression type in the list is working error free
				continue
			}
			return nil // Successfully compressed
		}
	}
	// No supported encoding found - send uncompressed
	return nil
}

func doCompression(resp *Response, compressType string) error {
	switch compressType {
	case "gzip":
		var b bytes.Buffer
		w := gzip.NewWriter(&b)

		// Write data to the gzip writer; it gets compressed into 'b'
		if _, err := w.Write(resp.Body); err != nil {
			return err
		}

		// Close the writer to flush any buffered data and write the GZIP footer
		if err := w.Close(); err != nil {
			return err
		}

		// Replace response body with compressed data
		resp.Body = b.Bytes()
		resp.SetHeader("Content-Encoding", compressType)
	}
	return nil
}
