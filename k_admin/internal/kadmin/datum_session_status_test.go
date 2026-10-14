package kadmin

import (
	"errors"
	"net/http"
	"testing"
)

func TestDatumDisabledUserRejectsExistingSessions(t *testing.T) {
	for _, route := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/datum/user/getInfo"},
		{http.MethodPost, "/datum/file"},
		{http.MethodPost, "/datum/ebook/upload"},
		{http.MethodGet, "/datum/download/file?fileId=1001"},
		{http.MethodGet, "/datum/download/ebook?ebookId=1001"},
		{http.MethodGet, "/system/notification/user/scroll"},
		{http.MethodDelete, "/datum/ebook/1001"},
		{http.MethodDelete, "/datum/ebook/favorite/1001"},
	} {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			store, db := newDatumAuthStore(t, nil)
			seedDatumUser(t, db, "alice", "secret5", "1", false)
			engine := datumEngine(t, store)
			adminToken := adminLoginForAudit(t, store)
			tokens := make([]string, 2)
			for i := range tokens {
				var err error
				tokens[i], _, err = store.datum.IssueSession(7)
				if err != nil {
					t.Fatal(err)
				}
				if rec := datumJSON(t, engine, http.MethodGet, "/datum/user/getInfo", tokens[i], nil); rec.Code != http.StatusOK {
					t.Fatalf("enabled session: status %d body %s", rec.Code, rec.Body.String())
				}
			}
			if rec := datumJSON(t, engine, http.MethodPut, "/datum/user", adminToken, map[string]interface{}{"userId": 7, "status": "0"}); rec.Code != http.StatusOK {
				t.Fatalf("disable account: status %d body %s", rec.Code, rec.Body.String())
			}
			for _, token := range tokens {
				if rec := datumJSON(t, engine, route.method, route.path, token, nil); rec.Code != http.StatusUnauthorized {
					t.Fatalf("disabled session: status %d body %s", rec.Code, rec.Body.String())
				}
				if _, err := store.datum.ResolveSession(token); !errors.Is(err, errDatumSessionInvalid) {
					t.Fatalf("disabled session was not revoked: %v", err)
				}
			}
			if len(db.files) != 0 || len(db.downloads) != 0 || db.ebookDownloads != 0 || db.users["alice"].Count != 3 {
				t.Fatal("disabled request changed user resources")
			}
			if rec := datumJSON(t, engine, http.MethodPut, "/datum/user", adminToken, map[string]interface{}{"userId": 7, "status": "1"}); rec.Code != http.StatusOK {
				t.Fatalf("enable account: status %d body %s", rec.Code, rec.Body.String())
			}
			if rec := datumJSON(t, engine, http.MethodGet, "/datum/user/getInfo", tokens[0], nil); rec.Code != http.StatusUnauthorized {
				t.Fatalf("revoked session revived after enable: status %d", rec.Code)
			}
			seedDatumCaptcha(t, store, "cap-enabled")
			if rec := datumJSON(t, engine, http.MethodPost, "/datum/user/login", "", map[string]string{
				"username": "alice", "password": "secret5", "uuid": "cap-enabled", "code": "123456",
			}); rec.Code != http.StatusOK {
				t.Fatalf("login after enable: status %d body %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestDatumDeletedUserRejectsExistingSession(t *testing.T) {
	store, db := newDatumAuthStore(t, nil)
	seedDatumUser(t, db, "alice", "secret5", "1", false)
	token, _, err := store.datum.IssueSession(7)
	if err != nil {
		t.Fatal(err)
	}
	delete(db.users, "alice")
	if rec := datumJSON(t, datumEngine(t, store), http.MethodGet, "/datum/user/getInfo", token, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("deleted session: status %d body %s", rec.Code, rec.Body.String())
	}
	if _, err := store.datum.ResolveSession(token); !errors.Is(err, errDatumSessionInvalid) {
		t.Fatalf("deleted session was not revoked: %v", err)
	}
}

type unavailableDatumAccountDB struct {
	*fakeDatumDB
}

func (d unavailableDatumAccountDB) Query(_ string, _ ...interface{}) ([]map[string]interface{}, error) {
	return nil, errors.New("database unavailable")
}

func TestDatumAccountLookupFailurePreservesSession(t *testing.T) {
	store, db := newDatumAuthStore(t, nil)
	seedDatumUser(t, db, "alice", "secret5", "1", false)
	token, _, err := store.datum.IssueSession(7)
	if err != nil {
		t.Fatal(err)
	}
	engine := datumEngine(t, store)
	store.conn = unavailableDatumAccountDB{db}
	if rec := datumJSON(t, engine, http.MethodGet, "/datum/user/getInfo", token, nil); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("database failure: status %d body %s", rec.Code, rec.Body.String())
	}
	store.conn = db
	if rec := datumJSON(t, engine, http.MethodGet, "/datum/user/getInfo", token, nil); rec.Code != http.StatusOK {
		t.Fatalf("database recovery: status %d body %s", rec.Code, rec.Body.String())
	}
}
