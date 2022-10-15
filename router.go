package simplehttp

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const (
	MethodGET    = "GET"
	MethodPOST   = "POST"
	MethodPUT    = "PUT"
	MethodDELETE = "DELETE"

	// pseudo:

	// MethodAny matches any method
	MethodAny = "~ANY~"
)

// Router is a Handler that preforms http requests routing.
type Router interface {
	Handler

	// Handle registers the handler for the given pattern.
	// NOTE: the last handler should be the real handler
	// and the previous handlers should be the middlewares.
	Handle(method string, path string, handlers ...Handler)

	// HandleFunc is a shortcut for router.Handle(method, path, handlers)
	// while handlers will be converted to Handler from HandlerFunc.
	// This is somewhat convenient for users, IMHO.
	HandleFunc(method string, path string, handlers ...HandlerFunc)

	// GET is a shortcut for router.HandleFunc("GET", path, handlers...)
	GET(path string, handlers ...HandlerFunc)

	// POST is a shortcut for router.HandleFunc("POST", path, handlers...)
	POST(path string, handlers ...HandlerFunc)

	// PUT is a shortcut for router.HandleFunc("PUT", path, handlers...)
	PUT(path string, handlers ...HandlerFunc)

	// DELETE is a shortcut for router.HandleFunc("DELETE", path, handlers...)
	DELETE(path string, handlers ...HandlerFunc)

	// TODO other methods

	// Use add middlewares to the router.
	// NOTE: middlewares added by Use will be executed BEFORE routing!
	Use(middlewares ...Handler)
}

type routerItem struct {
	method   string
	path     string
	handlers []Handler
}

// result of routerItem.match
const (
	missMatchMethod = -2
	missMatchPath   = -1
	matched         = 0
)

// match returns missMatchPath if the path is not matched,
// or missMatchMethod if the method is not matched while path is matched,
// or matched if both method and path are matched.
func (r *routerItem) match(method, path string) int {
	path = strings.TrimPrefix(path, "/")
	selfPath := strings.TrimPrefix(r.path, "/")
	if !pathHasPrefix(path, selfPath) {
		return missMatchPath
	}

	if r.method == MethodAny || r.method == method {
		return matched
	}

	return missMatchMethod
}

// pathHasPrefix returns true if the path is matched by the prefix.
// Give a prefix end with '/' to include sub paths and exclude
// the prefix before the last '/'.
//
// ，，这里英文好像写了有问题，不知道咋写，直接看例子：
//
//	pathHasPrefix("/abc", "/abc") == true
//	pathHasPrefix("/abc", "/abc/") == false
//	pathHasPrefix("/abc/def", "/abc/") == true
//	pathHasPrefix("/abc/def", "/abc") == false
func pathHasPrefix(path, prefix string) bool {
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	if prefix[len(prefix)-1] == '/' { // prefix end with '/': include sub path
		return true
	}
	// exclude sub path
	return len(path) == len(prefix) || path[len(prefix)] == '?'
}

// prefixRouter is a simple implementation of Router
//
// route "/abc" == "/abc/" will match "/abc", "/abc/", "/abc?x=1" or "/abc/def"
type prefixRouter struct {
	baseURL     string
	routes      []routerItem
	middlewares []Handler
}

// NewPrefixRouter creates a new prefix router, that is, a router
// that matches the path by prefix:
//
//	"/abc"  will match "/abc" or "/abc?x=1";
//	"/abc/" will match "/abc/", "/abc/?x=1", "/abc/def" or "/abc/def/..."
func NewPrefixRouter(baseURL string) Router {
	return &prefixRouter{
		baseURL:     baseURL,
		routes:      []routerItem{},
		middlewares: []Handler{},
	}
}

// ServeHTTP do the prefix router work
func (p *prefixRouter) ServeHTTP(c *Context) {
	c.setChain(append(p.middlewares, HandlerFunc(p.doRouter)))
	c.Next()
}

// doRouter do the real router work.
// It's funny that doRouter is a middleware (HandlerFunc) that will
// be added to the chain:
//
//	[prefixRouter.middlewares..., doRouter, routerItem.handlers...]
func (p *prefixRouter) doRouter(c *Context) {
	// relative path
	url := c.Request.Url // http://www.example.com:8080/abc/def

	// 。。我错了，url 就直接是 /abc/def，不用正则走一波了。
	//path, ok := UrlPath(url) // /abc/def
	//if !ok {                 // bad url: wtf
	//	c.Response.SetStateLine(c.Request.Version, 400) // 400 Bad Request
	//	return
	//}
	path := url

	relativePath, ok := p.relativePath(path) // /def when baseURL is /abc
	if !ok {                                 // BaseURL not match: should not happen
		c.Response.SetStateLine(c.Request.Version, 404) // 404 Not Found. I prefer 500, to be honest.
		fmt.Printf("[prefixRouter] Strange: BaseURL not match: expect=%s, got=%s\n", p.baseURL, path)
		return
	}

	methodMissedFlag := false // path matched, but method not matched

	for _, r := range p.routes {
		switch r.match(c.Request.Method, relativePath) {
		case matched:
			c.handlers = append(c.handlers, r.handlers...)
			c.Next()
			return
		case missMatchMethod:
			methodMissedFlag = true
		default:
			continue
		}
	}

	if methodMissedFlag {
		c.Response.SetStateLine(c.Request.Version, 405) // 405 Method Not Allowed
	} else {
		c.Response.SetStateLine(c.Request.Version, 404) // 404 Not Found
	}
}

// Deprecated: 没必要, Request line 拿到的直接就是路径了
// UrlPath get the path from the url:
//
//	Input: http://www.example.com:8080/abc/def
//	Output: /abc/def (string) + true (bool)
//
// return ("", false) if the url is invalid
func UrlPath(url string) (string, bool) {
	httpUrlRegexp := regexp.MustCompile(`.*?\:\/{2}.*?(\/.*)`)
	group := httpUrlRegexp.FindStringSubmatch(url)
	if len(group) != 1 { // bad url
		return "", false
	}
	return group[0], true
}

// relativePath returns the relative path of the url to the baseURL
// of prefixRouter. Return the trimmed url or ("", false) if the url do not
// match the baseURL.
func (p *prefixRouter) relativePath(path string) (string, bool) {
	if !strings.HasPrefix(path, p.baseURL) {
		return "", false
	}
	return strings.TrimPrefix(path, p.baseURL), true
}

// Handle adds a route to the router.
//
// I'm tired of writing this, just let it naive.
// TODO: Optimize me! max heap?
func (p *prefixRouter) Handle(method string, path string, handlers ...Handler) {
	if len(handlers) == 0 {
		panic("no handler")
	}

	for _, r := range p.routes {
		if r.method == method && r.path == path {
			panic("duplicate route")
		}
	}

	p.routes = append(p.routes, routerItem{
		method:   method,
		path:     path,
		handlers: handlers,
	})

	sort.Slice(p.routes, func(i, j int) bool {
		li := len(p.routes[i].path)
		lj := len(p.routes[j].path)
		if li == lj { // total order
			if p.routes[i].method == p.routes[j].method {
				return p.routes[i].path < p.routes[j].path
			}
			return p.routes[i].method < p.routes[j].method
		}
		return li > lj
	})
}

// HandleFunc is a shortcut for router.Handle(method, path, handlers)
// while handlers will be converted to Handler from HandlerFunc.
func (p *prefixRouter) HandleFunc(method string, path string, handlers ...HandlerFunc) {
	hs := make([]Handler, len(handlers))
	for i, h := range handlers {
		hs[i] = h
	}
	p.Handle(method, path, hs...)
}

func (p *prefixRouter) GET(path string, handlers ...HandlerFunc) {
	p.HandleFunc(MethodGET, path, handlers...)
}

func (p *prefixRouter) POST(path string, handlers ...HandlerFunc) {
	p.HandleFunc(MethodPOST, path, handlers...)
}

func (p *prefixRouter) PUT(path string, handlers ...HandlerFunc) {
	p.HandleFunc(MethodPUT, path, handlers...)
}

func (p *prefixRouter) DELETE(path string, handlers ...HandlerFunc) {
	p.HandleFunc(MethodDELETE, path, handlers...)
}

// Use adds middlewares to the router.
// NOTE: middlewares added by Use will be executed BEFORE routing!
func (p *prefixRouter) Use(middlewares ...Handler) {
	p.middlewares = append(p.middlewares, middlewares...)
}
