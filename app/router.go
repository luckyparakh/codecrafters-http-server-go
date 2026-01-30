package main

import (
	"sort"
	"strings"
)

type HandleFunc func(req *Request) *Response

type PrefixRoute struct {
	prefix  string
	handler HandleFunc
}

type Router struct {
	exactRoutes  map[string]HandleFunc
	prefixRoutes []PrefixRoute
}

func NewRouter() *Router {
	return &Router{
		exactRoutes:  make(map[string]HandleFunc),
		prefixRoutes: make([]PrefixRoute, 0),
	}
}

func (r *Router) RegisterExactRoute(path string, handler HandleFunc) {
	r.exactRoutes[path] = handler
}

func (r *Router) RegisterPrefixRoute(prefix string, handler HandleFunc) {
	r.prefixRoutes = append(r.prefixRoutes, PrefixRoute{
		prefix:  prefix,
		handler: handler,
	})

	// Sort by length (longest first) for proper matching priority
	// This ensures /api/v2/users matches before /api/v2
	sort.Slice(r.prefixRoutes, func(i, j int) bool {
		return len(r.prefixRoutes[i].prefix) > len(r.prefixRoutes[j].prefix)
	})
}

func (r *Router) Match(path string) HandleFunc {
	/*
	   Matching strategy:
	   1. Try exact match first (fastest - O(1) map lookup)
	   2. Try prefix routes in order (longest to shortest)
	   3. Return 404 handler if no match

	   Why longest-first?
	     Given routes: /api/users and /api
	     Request: /api/users/123
	     Should match: /api/users (more specific)
	     Not: /api (less specific)

	   Time complexity:
	     Exact match: O(1)
	     Prefix match: O(n) where n = number of prefix routes
	     Can be optimized to O(log n) with trie data structure
	*/
	if handler, ok := r.exactRoutes[path]; ok {
		return handler
	}

	// Match the longest prefix route
	for _, route := range r.prefixRoutes {
		if strings.HasPrefix(path, route.prefix) {
			return route.handler
		}
	}
	return handleNotFound
}
