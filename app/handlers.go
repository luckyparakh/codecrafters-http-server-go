package main

import "net/http"

const echoPrefix = "/echo/"
const userAgentPrefix = "/user-agent/"

func handleNotFound(r *Request) *Response {
	return NewResponse(404, "Not found", nil)
}

func handleRoot(r *Request) *Response {
	return NewResponse(http.StatusOK, "OK", nil)
}

func handleEcho(r *Request) *Response {
	content := r.Path[len(echoPrefix)+1:]
	resp := NewResponse(http.StatusOK, "OK", []byte(content))
	resp.SetHeader("Content-Type", "text/plain")
	return resp
}

func handleUserAgent(r *Request) *Response {
	userAgent, ok := r.Headers["user-agent"]
	if !ok {
		return NewResponse(http.StatusBadRequest, "Not found", nil)
	}
	resp := NewResponse(http.StatusOK, "OK", []byte(userAgent))
	resp.SetHeader("Content-Type", "text/plain")
	return resp
}
