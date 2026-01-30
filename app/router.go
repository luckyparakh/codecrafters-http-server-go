package main

import "strings"

type HandleFunc func(req *Request) *Response

type Router struct {
	exactRoutes map[string]HandleFunc
}

func NewRouter() *Router {
	return &Router{
		exactRoutes: make(map[string]HandleFunc),
	}
}

func (r *Router) RegisterExactRoute(path string, handler HandleFunc) {
	r.exactRoutes[path] = handler
}

func (r *Router) Match(path string) HandleFunc {
	if handler, ok := r.exactRoutes[path]; ok {
		return handler
	}

	for route, handler := range r.exactRoutes {
		if strings.HasPrefix(path, route) {
			return handler
		}
	}

	return handleNotFound
}
