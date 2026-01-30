package main

import "strings"

type HandleFunc func(req *Request) *Response

type Router struct {
	exactRoutes  map[string]HandleFunc
	prefixRoutes map[string]HandleFunc
}

func NewRouter() *Router {
	return &Router{
		exactRoutes:  make(map[string]HandleFunc),
		prefixRoutes: make(map[string]HandleFunc),
	}
}

func (r *Router) RegisterExactRoute(path string, handler HandleFunc) {
	r.exactRoutes[path] = handler
}

func (r *Router) RegisterPrefixRoute(prefix string, handler HandleFunc) {
	r.prefixRoutes[prefix] = handler
}

func (r *Router) Match(path string) HandleFunc {
	if handler, ok := r.exactRoutes[path]; ok {
		return handler
	}

	// Match the longest prefix route
	longestMatchLen := 0
	var matchingHandler HandleFunc
	for route, handler := range r.prefixRoutes {
		if strings.HasPrefix(path, route) {
			if len(route) > longestMatchLen {
				longestMatchLen = len(route)
				matchingHandler = handler
			}
		}
	}

	if matchingHandler != nil {
		return matchingHandler
	}

	return handleNotFound
}
