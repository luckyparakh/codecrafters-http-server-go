package main

import (
	"net/http"
	"os"
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
	userAgent, ok := r.Headers[strings.ToLower("User-Agent")]
	if !ok {
		return NewResponse(http.StatusBadRequest, "Not Found", nil)
	}
	resp := NewResponse(http.StatusOK, "OK", []byte(userAgent))
	resp.SetHeader("Content-Type", "text/plain")
	return resp
}

func handleFiles(r *Request) *Response {
	directoryPath := os.Args[2]
	fileName := r.Path[len(filesPrefix):]
	fileContent, err := os.ReadFile(directoryPath + fileName)
	if err != nil {
		return NewResponse(http.StatusNotFound, "Not Found", nil)
	}
	resp := NewResponse(http.StatusOK, "OK", fileContent)
	resp.SetHeader("Content-Type", "application/octet-stream")
	return resp
}
