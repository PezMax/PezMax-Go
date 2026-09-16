package kadmin

import (
	"bytes"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
)

const (
	defaultCaptchaTTL              = 2 * time.Minute
	defaultLoginFailureThreshold   = 5
	defaultLoginIPFailureThreshold = 20
	idempotencyProcessingTTL       = 30 * time.Second
	idempotencyBodyLimit           = 32 << 20
	idempotencyResponseLimit       = 1 << 20
)

var (
	errCaptchaInvalid      = errors.New("invalid captcha")
	errIdempotencyConflict = errors.New("idempotency key conflicts with another request")
	errIdempotencyRunning  = errors.New("request with this idempotency key is still running")
)

type securityPolicy struct {
	CaptchaEnabled          bool
	CaptchaTTL              time.Duration
	LoginLockEnabled        bool
	LoginFailureThreshold   int
	LoginIPFailureThreshold int
	LoginFailureWindow      time.Duration
	LoginLockDuration       time.Duration
	LoginIPWhitelist        []string
	IdempotencyTTL          time.Duration
}

type captchaChallenge struct {
	ID        string `json:"id"`
	Image     string `json:"image"`
	ExpiresIn int    `json:"expiresIn"`
}

type idempotencyStoredResponse struct {
	Body        string `json:"body,omitempty"`
	ContentType string `json:"contentType,omitempty"`
	Digest      string `json:"digest"`
	Owner       string `json:"owner"`
	State       string `json:"state"`
	Status      int    `json:"status,omitempty"`
}

type idempotencyReservation struct {
	key        string
	processing string
	digest     string
}

type idempotencyBeginResult struct {
	reservation *idempotencyReservation
	replay      *idempotencyStoredResponse
}

type redisDoer interface {
	do(...string) (interface{}, error)
}

type securityService struct {
	keyPrefix string
	redis     redisDoer
	secret    []byte
}

func newSecurityService(auth *authService) *securityService {
	return &securityService{
		keyPrefix: auth.keyPrefix + ":security",
		redis:     auth.redis,
		secret:    auth.secret,
	}
}

func (s *Store) loadSecurityPolicy() securityPolicy {
	values, err := s.readSystemConfig()
	if err != nil {
		values = defaultSystemConfigValues()
	}
	return securityPolicy{
		CaptchaEnabled:          configBool(values, "security.captcha_enabled", false),
		CaptchaTTL:              time.Duration(configInt(values, "security.captcha_ttl_seconds", 120, 30, 600)) * time.Second,
		LoginLockEnabled:        configBool(values, "security.login_lock_enabled", true),
		LoginFailureThreshold:   configInt(values, "security.login_failure_threshold", defaultLoginFailureThreshold, 2, 20),
		LoginIPFailureThreshold: configInt(values, "security.login_ip_failure_threshold", defaultLoginIPFailureThreshold, 5, 100),
		LoginFailureWindow:      time.Duration(configInt(values, "security.login_failure_window_minutes", 15, 1, 1440)) * time.Minute,
		LoginLockDuration:       time.Duration(configInt(values, "security.login_lock_minutes", 15, 1, 1440)) * time.Minute,
		LoginIPWhitelist:        splitConfigList(values["security.login_ip_whitelist"]),
		IdempotencyTTL:          time.Duration(configInt(values, "security.idempotency_ttl_seconds", 300, 30, 86400)) * time.Second,
	}
}

func configBool(values map[string]string, key string, fallback bool) bool {
	value, ok := values[key]
	if !ok {
		return fallback
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func configInt(values map[string]string, key string, fallback, minimum, maximum int) int {
	value, err := strconv.Atoi(strings.TrimSpace(values[key]))
	if err != nil || value < minimum || value > maximum {
		return fallback
	}
	return value
}

func splitConfigList(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ';' || unicode.IsSpace(r) })
	result := make([]string, 0, len(fields))
	for _, field := range fields {
		if item := strings.TrimSpace(field); item != "" {
			result = append(result, item)
		}
	}
	return result
}

func (s *securityService) issueCaptcha(ttl time.Duration) (captchaChallenge, error) {
	if ttl <= 0 {
		ttl = defaultCaptchaTTL
	}
	answer, err := randomCaptchaCode(6)
	if err != nil {
		return captchaChallenge{}, err
	}
	imageData, err := captchaPNG(answer)
	if err != nil {
		return captchaChallenge{}, err
	}
	id, err := randomHex(16)
	if err != nil {
		return captchaChallenge{}, err
	}
	if err := s.storeCaptcha(id, answer, ttl); err != nil {
		return captchaChallenge{}, err
	}
	return captchaChallenge{ID: id, Image: imageData, ExpiresIn: int(ttl.Seconds())}, nil
}

func (s *securityService) storeCaptcha(id, answer string, ttl time.Duration) error {
	digest := s.captchaDigest(id, answer)
	result, err := s.redis.do("SET", s.captchaKey(id), digest, "NX", "EX", durationSeconds(ttl))
	if err != nil {
		return err
	}
	if result != "OK" {
		return errors.New("captcha state was not stored")
	}
	return nil
}

func randomCaptchaCode(length int) (string, error) {
	const alphabet = "0123456789"
	if length <= 0 {
		return "", errors.New("captcha length must be positive")
	}
	code := make([]byte, length)
	for index := range code {
		value, err := secureRandomInt(len(alphabet))
		if err != nil {
			return "", err
		}
		code[index] = alphabet[value]
	}
	return string(code), nil
}

var captchaDigitGlyphs = [10][7]uint8{
	{14, 17, 19, 21, 25, 17, 14},
	{4, 12, 4, 4, 4, 4, 14},
	{14, 17, 1, 2, 4, 8, 31},
	{30, 1, 1, 14, 1, 1, 30},
	{2, 6, 10, 18, 31, 2, 2},
	{31, 16, 16, 30, 1, 1, 30},
	{14, 16, 16, 30, 17, 17, 14},
	{31, 1, 2, 4, 8, 8, 8},
	{14, 17, 17, 14, 17, 17, 14},
	{14, 17, 17, 15, 1, 1, 14},
}

func captchaPNG(answer string) (string, error) {
	const width, height, scale = 190, 56, 4
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: color.RGBA{R: 246, G: 248, B: 251, A: 255}}, image.Point{}, draw.Src)

	for range 9 {
		x1, err := secureRandomInt(width)
		if err != nil {
			return "", err
		}
		y1, err := secureRandomInt(height)
		if err != nil {
			return "", err
		}
		x2, err := secureRandomInt(width)
		if err != nil {
			return "", err
		}
		y2, err := secureRandomInt(height)
		if err != nil {
			return "", err
		}
		shade, err := secureRandomInt(35)
		if err != nil {
			return "", err
		}
		drawCaptchaLine(canvas, x1, y1, x2, y2, color.RGBA{R: uint8(165 + shade), G: uint8(175 + shade), B: uint8(190 + shade), A: 255})
	}

	foreground := [...]color.RGBA{
		{R: 25, G: 62, B: 110, A: 255},
		{R: 92, G: 38, B: 116, A: 255},
		{R: 28, G: 91, B: 73, A: 255},
		{R: 128, G: 53, B: 34, A: 255},
	}
	for index, character := range []byte(answer) {
		if character < '0' || character > '9' {
			return "", errors.New("captcha contains an unsupported character")
		}
		yJitter, err := secureRandomInt(9)
		if err != nil {
			return "", err
		}
		slant, err := secureRandomInt(3)
		if err != nil {
			return "", err
		}
		colorIndex, err := secureRandomInt(len(foreground))
		if err != nil {
			return "", err
		}
		glyph := captchaDigitGlyphs[character-'0']
		baseX := 10 + index*29
		baseY := 10 + yJitter
		for row, bits := range glyph {
			for column := range 5 {
				if bits&(1<<uint(4-column)) == 0 {
					continue
				}
				x := baseX + column*scale + (row-3)*(slant-1)
				y := baseY + row*scale
				draw.Draw(canvas, image.Rect(x, y, x+scale, y+scale), &image.Uniform{C: foreground[colorIndex]}, image.Point{}, draw.Src)
			}
		}
	}

	for range 140 {
		x, err := secureRandomInt(width)
		if err != nil {
			return "", err
		}
		y, err := secureRandomInt(height)
		if err != nil {
			return "", err
		}
		shade, err := secureRandomInt(80)
		if err != nil {
			return "", err
		}
		canvas.SetRGBA(x, y, color.RGBA{R: uint8(90 + shade), G: uint8(100 + shade), B: uint8(115 + shade), A: 255})
	}

	var encoded bytes.Buffer
	if err := png.Encode(&encoded, canvas); err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(encoded.Bytes()), nil
}

func drawCaptchaLine(canvas *image.RGBA, x1, y1, x2, y2 int, lineColor color.RGBA) {
	dx := absInt(x2 - x1)
	sx := -1
	if x1 < x2 {
		sx = 1
	}
	dy := -absInt(y2 - y1)
	sy := -1
	if y1 < y2 {
		sy = 1
	}
	err := dx + dy
	for {
		if image.Pt(x1, y1).In(canvas.Bounds()) {
			canvas.SetRGBA(x1, y1, lineColor)
		}
		if x1 == x2 && y1 == y2 {
			return
		}
		doubled := 2 * err
		if doubled >= dy {
			err += dy
			x1 += sx
		}
		if doubled <= dx {
			err += dx
			y1 += sy
		}
	}
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func (s *securityService) verifyCaptcha(id, answer string) error {
	id = strings.TrimSpace(id)
	answer = strings.ToUpper(strings.TrimSpace(answer))
	if id == "" || answer == "" {
		return errCaptchaInvalid
	}
	stored, err := s.redis.do("GETDEL", s.captchaKey(id))
	if errors.Is(err, errRedisNil) {
		return errCaptchaInvalid
	}
	if err != nil {
		return err
	}
	value, ok := stored.(string)
	if !ok || subtle.ConstantTimeCompare([]byte(value), []byte(s.captchaDigest(id, answer))) != 1 {
		return errCaptchaInvalid
	}
	return nil
}

func (s *securityService) captchaDigest(id, answer string) string {
	return base64.RawURLEncoding.EncodeToString(signHS256([]byte(id+":"+strings.ToUpper(strings.TrimSpace(answer))), s.secret))
}

func (s *securityService) captchaKey(id string) string {
	return s.keyPrefix + ":captcha:" + tokenHash(id)
}

func secureRandomInt(max int) (int, error) {
	if max <= 0 {
		return 0, errors.New("random maximum must be positive")
	}
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return 0, err
	}
	return int(binary.BigEndian.Uint64(raw[:]) % uint64(max)), nil
}

func (s *securityService) loginLocked(account, ip string, policy securityPolicy) (bool, error) {
	if !policy.LoginLockEnabled {
		return false, nil
	}
	accountLocked, err := s.redisExists(s.loginLockKey("account", normalizeAccount(account)))
	if err != nil || accountLocked {
		return accountLocked, err
	}
	if ipWhitelisted(ip, policy.LoginIPWhitelist) {
		return false, nil
	}
	return s.redisExists(s.loginLockKey("ip", strings.TrimSpace(ip)))
}

func (s *securityService) recordLoginFailure(account, ip string, policy securityPolicy) (bool, error) {
	if !policy.LoginLockEnabled {
		return false, nil
	}
	locked, err := s.incrementLoginFailure("account", normalizeAccount(account), policy.LoginFailureThreshold, policy)
	if err != nil {
		return false, err
	}
	if !ipWhitelisted(ip, policy.LoginIPWhitelist) {
		ipLocked, ipErr := s.incrementLoginFailure("ip", strings.TrimSpace(ip), policy.LoginIPFailureThreshold, policy)
		if ipErr != nil {
			return false, ipErr
		}
		locked = locked || ipLocked
	}
	return locked, nil
}

func (s *securityService) incrementLoginFailure(kind, value string, threshold int, policy securityPolicy) (bool, error) {
	script := `local count = redis.call('INCR', KEYS[1])
if count == 1 then redis.call('EXPIRE', KEYS[1], ARGV[1]) end
if count >= tonumber(ARGV[2]) then
  redis.call('SET', KEYS[2], '1', 'EX', ARGV[3])
  redis.call('DEL', KEYS[1])
  return 1
end
return 0`
	result, err := s.redis.do(
		"EVAL", script, "2",
		s.loginFailureKey(kind, value), s.loginLockKey(kind, value),
		durationSeconds(policy.LoginFailureWindow), strconv.Itoa(threshold), durationSeconds(policy.LoginLockDuration),
	)
	if err != nil {
		return false, err
	}
	locked, ok := result.(int64)
	return ok && locked == 1, nil
}

func (s *securityService) clearLoginFailures(account, ip string) error {
	keys := []string{s.loginFailureKey("account", normalizeAccount(account))}
	if strings.TrimSpace(ip) != "" {
		keys = append(keys, s.loginFailureKey("ip", strings.TrimSpace(ip)))
	}
	args := append([]string{"DEL"}, keys...)
	_, err := s.redis.do(args...)
	return err
}

func (s *securityService) unlockLogin(account, ip string) error {
	keys := []string{
		s.loginFailureKey("account", normalizeAccount(account)),
		s.loginLockKey("account", normalizeAccount(account)),
	}
	if strings.TrimSpace(ip) != "" {
		keys = append(keys, s.loginFailureKey("ip", strings.TrimSpace(ip)), s.loginLockKey("ip", strings.TrimSpace(ip)))
	}
	args := append([]string{"DEL"}, keys...)
	_, err := s.redis.do(args...)
	return err
}

func (s *securityService) redisExists(key string) (bool, error) {
	result, err := s.redis.do("EXISTS", key)
	if err != nil {
		return false, err
	}
	value, ok := result.(int64)
	if !ok {
		return false, fmt.Errorf("unexpected redis EXISTS reply: %T", result)
	}
	return value == 1, nil
}

func normalizeAccount(account string) string {
	return strings.ToLower(strings.TrimSpace(account))
}

func (s *securityService) loginFailureKey(kind, value string) string {
	return fmt.Sprintf("%s:login-failure:%s:%s", s.keyPrefix, kind, tokenHash(value))
}

func (s *securityService) loginLockKey(kind, value string) string {
	return fmt.Sprintf("%s:login-lock:%s:%s", s.keyPrefix, kind, tokenHash(value))
}

func ipWhitelisted(ip string, whitelist []string) bool {
	parsedIP := net.ParseIP(strings.TrimSpace(ip))
	for _, item := range whitelist {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if strings.EqualFold(item, strings.TrimSpace(ip)) {
			return true
		}
		if _, network, err := net.ParseCIDR(item); err == nil && parsedIP != nil && network.Contains(parsedIP) {
			return true
		}
	}
	return false
}

func (s *Store) idempotencyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !requiresIdempotency(c.Request.Method, c.Request.URL.Path) {
			c.Next()
			return
		}
		identity, status, identityErr := s.idempotencyIdentity(c)
		if identityErr != nil {
			fail(c, status, identityErr.Error())
			c.Abort()
			return
		}
		key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
		if !validIdempotencyKey(key) {
			fail(c, http.StatusBadRequest, "valid Idempotency-Key header is required")
			c.Abort()
			return
		}
		body, err := readRequestBody(c.Request, idempotencyBodyLimit)
		if err != nil {
			fail(c, http.StatusRequestEntityTooLarge, "request body is too large")
			c.Abort()
			return
		}
		digest := tokenHash(c.Request.Method + "\n" + c.Request.URL.RequestURI() + "\n" + string(body))
		policy := s.loadSecurityPolicy()
		result, err := s.security.beginIdempotency(identity, key, digest, idempotencyProcessingTTL)
		if err != nil {
			switch {
			case errors.Is(err, errIdempotencyConflict):
				fail(c, http.StatusConflict, err.Error())
			case errors.Is(err, errIdempotencyRunning):
				fail(c, http.StatusConflict, err.Error())
			default:
				fail(c, http.StatusServiceUnavailable, "idempotency storage unavailable")
			}
			c.Abort()
			return
		}
		if result.replay != nil {
			replayBody, decodeErr := base64.StdEncoding.DecodeString(result.replay.Body)
			if decodeErr != nil {
				fail(c, http.StatusServiceUnavailable, "stored idempotency response is invalid")
				c.Abort()
				return
			}
			c.Header("X-Idempotent-Replay", "true")
			c.Data(result.replay.Status, result.replay.ContentType, replayBody)
			c.Abort()
			return
		}

		stopRenewal := s.security.keepIdempotencyAlive(*result.reservation, idempotencyProcessingTTL)
		defer stopRenewal()
		capture := &boundedResponseWriter{ResponseWriter: c.Writer, limit: idempotencyResponseLimit}
		c.Writer = capture
		c.Next()
		if c.Writer.Status() >= http.StatusInternalServerError || capture.overflow {
			_ = s.security.releaseIdempotency(*result.reservation)
			return
		}
		if err := s.security.completeIdempotency(*result.reservation, c.Writer.Status(), c.Writer.Header().Get("Content-Type"), capture.body.Bytes(), policy.IdempotencyTTL); err != nil {
			_ = s.security.releaseIdempotency(*result.reservation)
		}
	}
}

func (s *securityService) beginIdempotency(identity, key, digest string, processingTTL time.Duration) (idempotencyBeginResult, error) {
	owner, err := randomHex(16)
	if err != nil {
		return idempotencyBeginResult{}, err
	}
	storedKey := s.keyPrefix + ":idempotency:" + tokenHash(identity+":"+key)
	processing := idempotencyStoredResponse{Digest: digest, Owner: owner, State: "processing"}
	encoded, _ := json.Marshal(processing)
	result, err := s.redis.do("SET", storedKey, string(encoded), "NX", "EX", durationSeconds(processingTTL))
	if err == nil && result == "OK" {
		return idempotencyBeginResult{reservation: &idempotencyReservation{key: storedKey, processing: string(encoded), digest: digest}}, nil
	}
	if err != nil && !errors.Is(err, errRedisNil) {
		return idempotencyBeginResult{}, err
	}
	existingRaw, getErr := s.redis.do("GET", storedKey)
	if errors.Is(getErr, errRedisNil) {
		return s.beginIdempotency(identity, key, digest, processingTTL)
	}
	if getErr != nil {
		return idempotencyBeginResult{}, getErr
	}
	existingText, ok := existingRaw.(string)
	if !ok {
		return idempotencyBeginResult{}, errors.New("invalid idempotency state")
	}
	var existing idempotencyStoredResponse
	if json.Unmarshal([]byte(existingText), &existing) != nil || existing.Digest == "" {
		return idempotencyBeginResult{}, errors.New("invalid idempotency state")
	}
	if existing.Digest != digest {
		return idempotencyBeginResult{}, errIdempotencyConflict
	}
	if existing.State == "done" {
		return idempotencyBeginResult{replay: &existing}, nil
	}
	return idempotencyBeginResult{}, errIdempotencyRunning
}

func (s *securityService) keepIdempotencyAlive(reservation idempotencyReservation, ttl time.Duration) func() {
	stop := make(chan struct{})
	done := make(chan struct{})
	var once sync.Once
	go func() {
		defer close(done)
		interval := ttl / 3
		if interval < time.Second {
			interval = time.Second
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				if err := s.renewIdempotency(reservation, ttl); err != nil {
					return
				}
			}
		}
	}()
	return func() {
		once.Do(func() { close(stop) })
		<-done
	}
}

func (s *securityService) renewIdempotency(reservation idempotencyReservation, ttl time.Duration) error {
	script := `if redis.call('GET', KEYS[1]) == ARGV[1] then return redis.call('EXPIRE', KEYS[1], ARGV[2]) end return 0`
	result, err := s.redis.do("EVAL", script, "1", reservation.key, reservation.processing, durationSeconds(ttl))
	if err != nil {
		return err
	}
	renewed, _ := result.(int64)
	if renewed != 1 {
		return errors.New("idempotency reservation expired")
	}
	return nil
}

func (s *securityService) completeIdempotency(reservation idempotencyReservation, status int, contentType string, body []byte, ttl time.Duration) error {
	completed := idempotencyStoredResponse{
		Body:        base64.StdEncoding.EncodeToString(body),
		ContentType: contentType,
		Digest:      reservation.digest,
		State:       "done",
		Status:      status,
	}
	encoded, _ := json.Marshal(completed)
	script := `if redis.call('GET', KEYS[1]) == ARGV[1] then
  redis.call('SET', KEYS[1], ARGV[2], 'EX', ARGV[3])
  return 1
end
return 0`
	result, err := s.redis.do("EVAL", script, "1", reservation.key, reservation.processing, string(encoded), durationSeconds(ttl))
	if err != nil {
		return err
	}
	updated, _ := result.(int64)
	if updated != 1 {
		return errors.New("idempotency reservation expired")
	}
	return nil
}

func (s *securityService) releaseIdempotency(reservation idempotencyReservation) error {
	script := `if redis.call('GET', KEYS[1]) == ARGV[1] then return redis.call('DEL', KEYS[1]) end return 0`
	_, err := s.redis.do("EVAL", script, "1", reservation.key, reservation.processing)
	return err
}

var (
	idempotentRouteRegistryMu sync.RWMutex
	idempotentRouteRegistry   = make(map[string]struct{})
)

// RegisterIdempotentCreateRoute marks POST /api/<prefix> as a high-risk
// create operation that requires the Idempotency-Key header, so generated
// CRUD modules reuse the existing duplicate-submission protection without
// editing the built-in whitelist. Built-in whitelist entries keep their
// existing behavior and take precedence. Registration is intended for
// startup wiring before the server begins serving requests.
func RegisterIdempotentCreateRoute(prefix string) {
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	if prefix == "" || strings.Contains(prefix, "/") {
		panic(fmt.Sprintf("invalid idempotent create route prefix %q: must be a single path segment", prefix))
	}
	idempotentRouteRegistryMu.Lock()
	defer idempotentRouteRegistryMu.Unlock()
	idempotentRouteRegistry[prefix] = struct{}{}
}

func requiresIdempotency(method, path string) bool {
	if method != http.MethodPost {
		return false
	}
	segments := splitAPIPath(path)
	switch {
	case len(segments) == 1 && segments[0] == "users":
		return true
	case len(segments) == 2 && segments[0] == "users" && segments[1] == "import":
		return true
	case len(segments) == 2 && segments[0] == "rbac" && (segments[1] == "departments" || segments[1] == "roles"):
		return true
	case len(segments) == 1 && segments[0] == "admin-menus":
		return true
	case len(segments) == 2 && segments[0] == "dictionaries" && (segments[1] == "types" || segments[1] == "data"):
		return true
	case len(segments) == 1 && segments[0] == "jobs":
		return true
	case len(segments) == 3 && segments[0] == "jobs" && segments[2] == "run":
		return true
	case len(segments) == 3 && segments[0] == "codegen" && segments[1] == "tables" && segments[2] == "import":
		return true
	case len(segments) == 4 && segments[0] == "codegen" && segments[1] == "configs" && segments[3] == "generate":
		return true
	case len(segments) == 1 && segments[0] == "notifications":
		return true
	case len(segments) == 1:
		idempotentRouteRegistryMu.RLock()
		_, registered := idempotentRouteRegistry[segments[0]]
		idempotentRouteRegistryMu.RUnlock()
		return registered
	default:
		return false
	}
}

func validIdempotencyKey(value string) bool {
	if len(value) < 8 || len(value) > 128 {
		return false
	}
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("-_.:", r) {
			continue
		}
		return false
	}
	return true
}

func (s *Store) idempotencyIdentity(c *gin.Context) (string, int, error) {
	token := tokenFromRequest(c)
	claims, err := s.auth.parseAccessToken(token)
	if err != nil {
		return "", http.StatusUnauthorized, errors.New("invalid token")
	}
	blacklisted, err := s.auth.isAccessTokenBlacklisted(claims.JTI)
	if err != nil {
		return "", http.StatusServiceUnavailable, errors.New("auth storage unavailable")
	}
	if blacklisted {
		return "", http.StatusUnauthorized, errors.New("invalid token")
	}
	enabled, err := s.userAccountEnabled(claims.UserID)
	if err != nil {
		return "", http.StatusInternalServerError, err
	}
	if !enabled {
		return "", http.StatusForbidden, errors.New("account disabled")
	}
	c.Set("vben_user_id", claims.UserID)
	return "user:" + strconv.FormatInt(claims.UserID, 10), 0, nil
}

func readRequestBody(request *http.Request, limit int64) ([]byte, error) {
	if request.Body == nil {
		return []byte{}, nil
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, limit+1))
	if err != nil {
		return nil, err
	}
	request.Body = io.NopCloser(bytes.NewReader(body))
	if int64(len(body)) > limit {
		return nil, errors.New("request body exceeds limit")
	}
	return body, nil
}

type boundedResponseWriter struct {
	gin.ResponseWriter
	body     bytes.Buffer
	limit    int
	overflow bool
}

func (w *boundedResponseWriter) Write(data []byte) (int, error) {
	if !w.overflow {
		remaining := w.limit - w.body.Len()
		if len(data) <= remaining {
			_, _ = w.body.Write(data)
		} else {
			w.overflow = true
			w.body.Reset()
		}
	}
	return w.ResponseWriter.Write(data)
}

func (w *boundedResponseWriter) WriteString(data string) (int, error) {
	return w.Write([]byte(data))
}

func splitAPIPath(path string) []string {
	path = strings.Trim(strings.TrimPrefix(path, "/api"), "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}

func durationSeconds(value time.Duration) string {
	seconds := int(value.Seconds())
	if seconds < 1 {
		seconds = 1
	}
	return strconv.Itoa(seconds)
}
