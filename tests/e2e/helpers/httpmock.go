//go:build e2e

package helpers

import (
	"net/http"
	"net/http/httptest"
	"regexp"
)

type Route struct {
	Match   func(*http.Request) bool
	Handler http.HandlerFunc
}

func MatchExact(method, path string) func(*http.Request) bool {
	return func(r *http.Request) bool {
		return r.Method == method && r.URL.Path == path
	}
}

func MatchRegex(method string, re *regexp.Regexp) func(*http.Request) bool {
	return func(r *http.Request) bool {
		return r.Method == method && re.MatchString(r.URL.Path)
	}
}

func NewServer(routes ...Route) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, rt := range routes {
			if rt.Match(r) {
				rt.Handler(w, r)
				return
			}
		}
		http.NotFound(w, r)
	}))
}
