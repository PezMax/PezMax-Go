package loadrank

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"
)

// RouteIndex resolves an observed request path to the registered route
// template. Templates keep the summary-table cardinality bounded: a resource
// like /api/users/123 and /api/users/456 share one row instead of exploding
// the route dimension.
type RouteIndex struct {
	entries []routeEntry
}

type routeEntry struct {
	method   string
	segments []string
	template string
}

// NewRouteIndex snapshots the registered Gin routes. Generated interfaces are
// detected automatically as long as they are registered as normal Gin routes
// (e.g. /api/orders/:id) inside the KAdmin API group.
func NewRouteIndex(routes []gin.RouteInfo) *RouteIndex {
	index := &RouteIndex{entries: make([]routeEntry, 0, len(routes))}
	for _, route := range routes {
		template := ginTemplateToDisplay(route.Path)
		if template == "" {
			continue
		}
		index.entries = append(index.entries, routeEntry{
			method:   strings.ToUpper(route.Method),
			segments: splitPath(template),
			template: template,
		})
	}
	return index
}

// Resolve returns the display template for the request, or "" when no
// registered route matches (the caller falls back to heuristic
// normalization).
func (i *RouteIndex) Resolve(method string, path string) string {
	if i == nil {
		return ""
	}
	method = strings.ToUpper(method)
	segments := splitPath(path)
	for _, entry := range i.entries {
		if entry.method != method || !matchesTemplate(entry.segments, segments) {
			continue
		}
		return entry.template
	}
	return ""
}

// matchesTemplate compares concrete path segments against a template where
// "{param}" matches one segment and "{param...}" matches the remainder.
func matchesTemplate(template []string, concrete []string) bool {
	// A trailing "{param...}" absorbs any number of remaining segments.
	if len(template) > 0 && strings.HasSuffix(template[len(template)-1], "...}") {
		if len(concrete) < len(template)-1 {
			return false
		}
		prefix := template[:len(template)-1]
		return equalSegments(prefix, concrete[:len(prefix)])
	}
	if len(template) != len(concrete) {
		return false
	}
	return equalSegments(template, concrete)
}

func equalSegments(template []string, concrete []string) bool {
	for index := range template {
		if strings.HasPrefix(template[index], "{") && strings.HasSuffix(template[index], "}") {
			continue
		}
		if template[index] != concrete[index] {
			return false
		}
	}
	return true
}

// ginTemplateToDisplay converts a Gin route pattern (/api/users/:id,
// /files/*path) into a display template (/api/users/{id}, /files/{path...}).
func ginTemplateToDisplay(path string) string {
	if path == "" {
		return ""
	}
	segments := splitPath(path)
	converted := make([]string, len(segments))
	for index, segment := range segments {
		switch {
		case strings.HasPrefix(segment, ":") && len(segment) > 1:
			converted[index] = "{" + segment[1:] + "}"
		case strings.HasPrefix(segment, "*") && len(segment) > 1:
			converted[index] = "{" + segment[1:] + "...}"
		default:
			converted[index] = segment
		}
	}
	return "/" + strings.Join(converted, "/")
}

// normalizeRouteTemplate is the fallback when no registered template matches
// (unmatched paths, static files). Variable segments are heuristically
// collapsed so the route dimension stays bounded.
func normalizeRouteTemplate(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	segments := splitPath(path)
	for index, segment := range segments {
		if isVariableSegment(segment) {
			segments[index] = "{id}"
		}
	}
	normalized := "/" + strings.Join(segments, "/")
	return truncateRoute(normalized)
}

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func isVariableSegment(segment string) bool {
	if segment == "" || len(segment) > 64 {
		return len(segment) > 64
	}
	if uuidPattern.MatchString(segment) {
		return true
	}
	for _, r := range segment {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// splitPath splits a request path into segments, dropping empty parts and a
// trailing slash so "/api/users" and "/api/users/" resolve identically.
func splitPath(path string) []string {
	parts := strings.Split(path, "/")
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			segments = append(segments, part)
		}
	}
	return segments
}

func truncateRoute(value string) string {
	runes := []rune(value)
	if len(runes) <= maxRouteLength {
		return value
	}
	return string(runes[:maxRouteLength])
}
