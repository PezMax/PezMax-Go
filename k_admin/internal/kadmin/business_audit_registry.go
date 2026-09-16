package kadmin

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
)

// AuditSnapshotLoader loads the current snapshot of a mutated resource for
// before/after diffing in the unified business audit middleware. It receives
// the id path segment and must return nil when the resource cannot be loaded.
// Generated CRUD modules pass a loader backed by their own repository.
type AuditSnapshotLoader func(resourceID string) interface{}

// businessAuditResolver resolves the business audit descriptor for a request
// whose first path segment has a registered convention.
type businessAuditResolver func(method string, parts []string) (businessAuditDescriptor, bool)

var (
	businessAuditRegistryMu sync.RWMutex
	businessAuditRegistry   = make(map[string]businessAuditResolver)
)

// RegisterBusinessAuditResource registers the standard single-table CRUD
// mutation conventions for the /api/<prefix> route group:
//
//	POST               /api/<prefix>      -> create
//	PUT | PATCH        /api/<prefix>/:id  -> update
//	DELETE             /api/<prefix>/:id  -> delete
//
// The loader supplies before/after snapshots for update and delete events; it
// is never consulted for creates. Registered conventions are checked before
// the built-in descriptors, and built-in conventions remain the fallback when
// the resolver does not match. Registration is intended for startup wiring
// before the server begins serving requests.
func RegisterBusinessAuditResource(prefix, resource string, loader AuditSnapshotLoader) {
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	if prefix == "" || strings.Contains(prefix, "/") {
		panic(fmt.Sprintf("invalid business audit prefix %q: must be a single path segment", prefix))
	}
	if resource = strings.TrimSpace(resource); resource == "" {
		panic("business audit resource name must not be empty")
	}
	businessAuditRegistryMu.Lock()
	defer businessAuditRegistryMu.Unlock()
	businessAuditRegistry[prefix] = func(method string, parts []string) (businessAuditDescriptor, bool) {
		switch {
		case len(parts) == 1 && method == http.MethodPost:
			return businessAuditDescriptor{Action: "create", Resource: resource, ResourceID: "new"}, true
		case len(parts) == 2 && (method == http.MethodPut || method == http.MethodPatch):
			return businessAuditDescriptor{Action: "update", Resource: resource, ResourceID: parts[1], Loader: loader}, true
		case len(parts) == 2 && method == http.MethodDelete:
			return businessAuditDescriptor{Action: "delete", Resource: resource, ResourceID: parts[1], Loader: loader}, true
		default:
			return businessAuditDescriptor{}, false
		}
	}
}

// businessAuditResolverFor returns the resolver registered for the first path
// segment, or nil when no convention was registered.
func businessAuditResolverFor(prefix string) businessAuditResolver {
	businessAuditRegistryMu.RLock()
	defer businessAuditRegistryMu.RUnlock()
	return businessAuditRegistry[prefix]
}
