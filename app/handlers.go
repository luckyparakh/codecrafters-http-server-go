package main

import (
	"net/http"
	"strings"
)

const echoPrefix = "/echo/"
const userAgentPrefix = "/user-agent"

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
