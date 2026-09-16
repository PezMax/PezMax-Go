package kadmin

import (
	"net/http"
	"testing"
)

func TestRegisterBusinessAuditResourceMapsCRUDConventions(t *testing.T) {
	loader := func(resourceID string) interface{} {
		return map[string]interface{}{"id": resourceID}
	}
	RegisterBusinessAuditResource("test-products", "test-product", loader)

	tests := []struct {
		method, path, resource, action, id string
		wantLoader                         bool
	}{
		{http.MethodPost, "/api/test-products", "test-product", "create", "new", false},
		{http.MethodPut, "/api/test-products/7", "test-product", "update", "7", true},
		{http.MethodPatch, "/api/test-products/8", "test-product", "update", "8", true},
		{http.MethodDelete, "/api/test-products/9", "test-product", "delete", "9", true},
	}
	for _, tt := range tests {
		descriptor, ok := describeBusinessMutation(tt.method, tt.path)
		if !ok {
			t.Fatalf("%s %s: expected a descriptor", tt.method, tt.path)
		}
		if descriptor.Resource != tt.resource || descriptor.Action != tt.action || descriptor.ResourceID != tt.id {
			t.Fatalf("%s %s descriptor = %#v", tt.method, tt.path, descriptor)
		}
		if tt.wantLoader != (descriptor.Loader != nil) {
			t.Fatalf("%s %s loader presence = %v, want %v", tt.method, tt.path, descriptor.Loader != nil, tt.wantLoader)
		}
	}
}

func TestRegisteredAuditConventionIgnoresUnmatchedShapes(t *testing.T) {
	RegisterBusinessAuditResource("test-widgets", "test-widget", nil)
	for _, tt := range []struct{ method, path string }{
		{http.MethodGet, "/api/test-widgets"},
		{http.MethodPost, "/api/test-widgets/1"},
		{http.MethodPut, "/api/test-widgets"},
		{http.MethodPut, "/api/test-widgets/1/extra"},
		{http.MethodDelete, "/api/test-widgets"},
	} {
		if descriptor, ok := describeBusinessMutation(tt.method, tt.path); ok {
			t.Fatalf("%s %s unexpectedly matched descriptor %#v", tt.method, tt.path, descriptor)
		}
	}
}

func TestBusinessAuditRegistryFallsBackToBuiltInConventions(t *testing.T) {
	RegisterBusinessAuditResource("test-fallback", "test-fallback", nil)
	descriptor, ok := describeBusinessMutation(http.MethodPost, "/api/users")
	if !ok || descriptor.Resource != "user" || descriptor.Action != "create" {
		t.Fatalf("built-in convention was shadowed: %#v, ok=%v", descriptor, ok)
	}
	if _, ok := describeBusinessMutation(http.MethodDelete, "/api/files/9"); !ok {
		t.Fatal("built-in file deletion convention was lost")
	}
	if _, ok := describeBusinessMutation(http.MethodPost, "/api/test-fallback"); !ok {
		t.Fatal("registered convention did not resolve its own prefix")
	}
}

func TestBusinessAuditSnapshotPrefersRegisteredLoader(t *testing.T) {
	received := ""
	descriptor := businessAuditDescriptor{
		Action:     "update",
		Resource:   "test-resource",
		ResourceID: "42",
		Loader: func(resourceID string) interface{} {
			received = resourceID
			return map[string]interface{}{"id": resourceID}
		},
	}
	store := &Store{}
	snapshot := store.loadBusinessAuditSnapshot(descriptor, "/api/test-resource/42")
	if received != "42" {
		t.Fatalf("loader received resource id %q, want %q", received, "42")
	}
	if snapshot == nil {
		t.Fatal("registered loader result was dropped")
	}
}

func TestBusinessAuditSnapshotFallsBackForNilLoader(t *testing.T) {
	descriptor := businessAuditDescriptor{
		Action:     "update",
		Resource:   "unregistered-resource",
		ResourceID: "42",
	}
	store := &Store{}
	if snapshot := store.loadBusinessAuditSnapshot(descriptor, "/api/unregistered-resource/42"); snapshot != nil {
		t.Fatalf("unknown resource without loader must not resolve a snapshot, got %#v", snapshot)
	}
}

func TestRegisterBusinessAuditResourceRejectsInvalidArguments(t *testing.T) {
	for _, prefix := range []string{"", "/", "a/b", " a/b "} {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatalf("expected panic for prefix %q", prefix)
				}
			}()
			RegisterBusinessAuditResource(prefix, "test-resource", nil)
		}()
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("expected panic for empty resource name")
			}
		}()
		RegisterBusinessAuditResource("test-resource", "  ", nil)
	}()
}
