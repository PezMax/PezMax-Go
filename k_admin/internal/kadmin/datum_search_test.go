package kadmin

import (
	"net/http"
	"testing"
)

func TestDatumFileSearchRejectsInvalidYearRanges(t *testing.T) {
	store, _ := newDatumAuthStore(t, nil)
	engine := datumEngine(t, store)
	for _, query := range []string{
		"yearFrom=hello", "yearTo=2101", "yearFrom=1899", "yearTo=2024.5", "yearFrom=2026&yearTo=2024",
	} {
		response := datumJSON(t, engine, http.MethodGet, "/datum/file/list?"+query, "", nil)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("query %q: got %d, body %s", query, response.Code, response.Body.String())
		}
	}
}
