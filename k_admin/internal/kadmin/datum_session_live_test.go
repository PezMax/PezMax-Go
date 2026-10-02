package kadmin

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/GoAdminGroup/go-admin/internal/kadmin/platform/storage"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

// Opt in against the local running server; this exercises the generated admin
// endpoint, real login, multipart upload, PostgreSQL and Redis. All resource
// mutations belong to one disposable account and are cleaned up afterwards.
func TestDatumLiveDisableAfterLogin(t *testing.T) {
	base := os.Getenv("KADMIN_TEST_LIVE_URL")
	if base == "" {
		t.Skip("set KADMIN_TEST_LIVE_URL to the local running backend")
	}
	parsed, err := url.Parse(base)
	if err != nil || net.ParseIP(parsed.Hostname()) == nil || !net.ParseIP(parsed.Hostname()).IsLoopback() {
		t.Fatal("live regression only accepts a loopback server")
	}
	values, err := godotenv.Read("../../.env")
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range values {
		if _, exists := os.LookupEnv(key); !exists {
			t.Setenv(key, value)
		}
	}
	dsn := &url.URL{Scheme: "postgres", Host: net.JoinHostPort(os.Getenv("KADMIN_DB_HOST"), os.Getenv("KADMIN_DB_PORT")), Path: os.Getenv("KADMIN_DB_NAME"), User: url.UserPassword(os.Getenv("KADMIN_DB_USER"), os.Getenv("KADMIN_DB_PASSWORD"))}
	dsn.RawQuery = "sslmode=disable"
	conn, err := sql.Open("postgres", dsn.String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.Ping(); err != nil {
		t.Fatal(err)
	}
	unique, err := randomHex(6)
	if err != nil {
		t.Fatal(err)
	}
	username := "codex_status_" + unique
	password := "Codex12!"
	hash, err := hashDatumSecret(password)
	if err != nil {
		t.Fatal(err)
	}
	var userID int64
	if err := conn.QueryRow(`INSERT INTO public.ptmj_user (user_name, password, status, count, create_time, update_time) VALUES ($1, $2, '0', 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP) RETURNING user_id`, username, hash).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	auth := newAuthServiceFromEnv()
	identity := newDatumIdentity(auth.keyPrefix+":datum", auth.redis, time.Hour)
	var sessionTokens []string
	defer func() {
		for _, token := range sessionTokens {
			_ = identity.RevokeSession(token)
		}
		rows, err := conn.Query(`SELECT file_url FROM public.ptmj_file WHERE user_id = $1`, userID)
		if err != nil {
			t.Error(err)
			return
		}
		var objectKeys []string
		for rows.Next() {
			var rawURL string
			if err := rows.Scan(&rawURL); err != nil {
				t.Error(err)
				continue
			}
			if objectURL, err := url.Parse(rawURL); err == nil {
				objectKeys = append(objectKeys, strings.TrimPrefix(objectURL.Path, "/"+os.Getenv("KADMIN_MINIO_BUCKET")+"/"))
			}
		}
		rows.Close()
		for _, query := range []string{
			`DELETE FROM public.ptmj_file_download WHERE user_id = $1`,
			`DELETE FROM public.ptmj_file WHERE user_id = $1`,
			`DELETE FROM public.ptmj_user WHERE user_id = $1`,
		} {
			if _, err := conn.Exec(query, userID); err != nil {
				t.Error(err)
			}
		}
		remote := storage.NewMinio(storage.MinioConfig{Endpoints: []string{os.Getenv("KADMIN_MINIO_ENDPOINT")}, AccessKey: os.Getenv("KADMIN_MINIO_ACCESS_KEY"), SecretKey: os.Getenv("KADMIN_MINIO_SECRET_KEY"), Bucket: os.Getenv("KADMIN_MINIO_BUCKET"), UseSSL: datumEnvBool("KADMIN_MINIO_USE_SSL"), Region: datumEnv("KADMIN_MINIO_REGION", "us-east-1"), Timeout: 10 * time.Second})
		for _, objectKey := range objectKeys {
			if err := remote.Delete(context.Background(), objectKey); err != nil {
				t.Error("cleanup test object:", err)
			}
		}
		(&Store{auth: auth, datum: identity}).invalidateDatumFiles()
	}()
	client := &http.Client{Timeout: 15 * time.Second}
	call := func(method, path, token, contentType string, body io.Reader) (int, []byte) {
		t.Helper()
		request, err := http.NewRequest(method, strings.TrimRight(base, "/")+path, body)
		if err != nil {
			t.Fatal(err)
		}
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		if contentType != "" {
			request.Header.Set("Content-Type", contentType)
		}
		request.Header.Set("Idempotency-Key", "codex-status-"+unique+"-"+strconv.FormatInt(time.Now().UnixNano(), 10))
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		payload, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		return response.StatusCode, payload
	}
	var adminID int64
	if err := conn.QueryRow(`SELECT id FROM public.goadmin_users WHERE status = 'enable' ORDER BY id LIMIT 1`).Scan(&adminID); err != nil {
		t.Fatal(err)
	}
	claims := accessTokenClaims{UserID: adminID, Type: "access", Issuer: auth.issuer, JTI: "codex-status-" + unique, IssuedAt: time.Now().Unix(), ExpiresAt: time.Now().Add(5 * time.Minute).Unix()}
	adminToken, err := auth.signAccessToken(claims)
	if err != nil {
		t.Fatal(err)
	}
	defer auth.blacklistAccessToken(claims)
	setStatus := func(status string) {
		t.Helper()
		payload, _ := json.Marshal(map[string]interface{}{"userName": username, "password": hash, "avatar": "", "count": 0, "status": status, "creatBy": "", "createTime": "", "updateBy": "", "updateTime": "", "remark": "local session regression"})
		code, body := call(http.MethodPut, fmt.Sprintf("/api/platform-users/%d", userID), adminToken, "application/json", bytes.NewReader(payload))
		if code != http.StatusOK {
			t.Fatalf("admin status %s: HTTP %d %s", status, code, body)
		}
	}
	setStatus("1")
	security := newSecurityService(auth)
	captchaID := "codex-status-" + unique
	if err := security.storeCaptcha(captchaID, "123456", time.Minute); err != nil {
		t.Fatal(err)
	}
	loginBody, _ := json.Marshal(map[string]string{"username": username, "password": password, "uuid": captchaID, "code": "123456"})
	code, body := call(http.MethodPost, "/datum/user/login", "", "application/json", bytes.NewReader(loginBody))
	if code != http.StatusOK {
		t.Fatalf("login after unban: HTTP %d %s", code, body)
	}
	var login struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &login); err != nil || login.Data.Token == "" {
		t.Fatal("login did not issue a session")
	}
	token := login.Data.Token
	sessionTokens = append(sessionTokens, token)
	upload := func() (int, []byte) {
		var data bytes.Buffer
		writer := multipart.NewWriter(&data)
		file, err := writer.CreateFormFile("file", username+".pdf")
		if err != nil {
			t.Fatal(err)
		}
		_, _ = file.Write([]byte("%PDF-1.4\n1 0 obj<</Type/Catalog>>endobj\n%%EOF\n"))
		for key, value := range map[string]string{"fileSubject": username, "fileSchool": "Regression", "fileYear": "2026", "fileType": "4"} {
			_ = writer.WriteField(key, value)
		}
		_ = writer.Close()
		return call(http.MethodPost, "/datum/file", token, writer.FormDataContentType(), &data)
	}
	code, body = upload()
	if code != http.StatusOK {
		t.Fatalf("upload before ban: HTTP %d %s", code, body)
	}
	t.Log("unban -> real login -> multipart upload: HTTP 200")
	setStatus("0")
	code, body = upload()
	if code != http.StatusUnauthorized {
		t.Fatalf("upload after admin ban: HTTP %d; expected 401 (running server still accepts disabled sessions)", code)
	}
	if !bytes.Contains(body, []byte("账号已被停用")) {
		t.Fatalf("ban response did not identify disabled account: %s", body)
	}
	for _, path := range []string{"/datum/user/getInfo", "/datum/download/file?fileId=1", "/system/notification/user/scroll"} {
		code, _ := call(http.MethodGet, path, token, "", nil)
		if code != http.StatusUnauthorized {
			t.Fatalf("disabled session %s: HTTP %d", path, code)
		}
	}
	t.Log("admin ban -> same-session upload/download/info/notification: HTTP 401")
}
