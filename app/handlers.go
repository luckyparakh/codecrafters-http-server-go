package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	echoPrefix      = "/echo/"
	userAgentPrefix = "/user-agent"
	filesPrefix     = "/files/"
)

func handleNotFound(r *Request) *Response {
	return NewResponse(http.StatusNotFound, "Not Found", nil)
}

func handleRoot(r *Request) *Response {
	return NewResponse(http.StatusOK, "OK", nil)
}

func handleEcho(r *Request) *Response {
	content := r.Path[len(echoPrefix):]
	resp := NewResponse(http.StatusOK, "OK", []byte(content))
	resp.SetHeader("Content-Type", "text/plain")
	return resp
}

func handleUserAgent(r *Request) *Response {
	userAgent, ok := r.GetHeader("User-Agent")
	if !ok {
		return NewResponse(http.StatusBadRequest, "Not Found", nil)
	}
	resp := NewResponse(http.StatusOK, "OK", []byte(userAgent))
	resp.SetHeader("Content-Type", "text/plain")
	return resp
}

func handleFiles(r *Request) *Response {
	fileName := r.Path[len(filesPrefix):]

	// if fileName is empty, return 400 Bad Request
	if fileName == "" {
		return NewResponse(http.StatusBadRequest, "Bad Request", []byte("File name is required"))
	}

	/*
			   Security: Normalize and validate filename to prevent path traversal attacks

			   filepath.Clean() normalizes the path:
			     "foo/../bar"     → "bar"
			     "foo/./bar"      → "foo/bar"
			     "foo//bar"       → "foo/bar"
			     "./secret"       → "secret"
			     "a/../../b"      → "../b"  (still contains .., caught by next check)

			   Why Clean() before checking for ".."?
			     Without Clean: "foo/./../bar" contains ".." → blocked ✓
			     But: "foo/./dummy/../bar" after Clean → "foo/bar" → allowed ✓
			     Attack: URL encoding could bypass simple string checks
				 // Attacker sends:
					fileName = "normalfile/./../../secret.txt"

					// Without Clean():
					strings.Contains(fileName, "..")  // → true ✓ Blocked

					// But what if attacker URL-encodes it?
					fileName = "normalfile/.%2F..%2Fsecret.txt"  // URL decoded after your check
					strings.Contains(fileName, "..")  // → false ✗ BYPASSED!

					// With Clean():
					fileName = filepath.Clean(fileName)  // → "../secret.txt"
					strings.Contains(fileName, "..") // → true ✓ Blocked

			   Defense in depth:
			     1. Clean() normalizes to canonical form
			     2. Check for ".." catches parent directory access
			     3. Check for "." prefix catches hidden files and current dir
			     4. Final absolute path verification ensures file is within allowed directory
	*/
	fileName = filepath.Clean(fileName)
	if strings.Contains(fileName, "..") || strings.HasPrefix(fileName, ".") {
		return NewResponse(http.StatusBadRequest, "Bad Request", []byte("Invalid file name"))
	}

	// Join with base directory
	fullPath := filepath.Join(dirPath, fileName)

	// Additional security: verify the resolved path is still within directory
	absFullPath, err := filepath.Abs(fullPath)
	if err != nil {
		return NewResponse(http.StatusInternalServerError, "Internal Server Error", []byte(err.Error()))
	}
	absDir, err := filepath.Abs(dirPath)
	if err != nil {
		return NewResponse(http.StatusInternalServerError, "Internal Server Error", []byte(err.Error()))
	}
	if !strings.HasPrefix(absFullPath, absDir) {
		return NewResponse(http.StatusBadRequest, "Bad Request", []byte("path traversal detected"))
	}

	switch r.Method {
	case http.MethodGet:
		fileContent, err := os.ReadFile(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				return NewResponse(http.StatusNotFound, "Not Found", []byte("File not found"))
			}
			return NewResponse(http.StatusInternalServerError, "Internal Server Error", []byte(err.Error()))
		}
		resp := NewResponse(http.StatusOK, "OK", fileContent)
		resp.SetHeader("Content-Type", "application/octet-stream")
		return resp
	case http.MethodPost:
		err := os.WriteFile(fullPath, r.Body, 0644)
		if err != nil {
			return NewResponse(http.StatusInternalServerError, "Internal Server Error", []byte(err.Error()))
		}
		return NewResponse(http.StatusCreated, "Created", nil)
	default:
		return NewResponse(http.StatusMethodNotAllowed, "Method Not Allowed", nil)
	}
}
