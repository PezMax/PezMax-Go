package kadmin

import (
	"bufio"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAccessTokenTTL  = 30 * time.Minute
	defaultRefreshTokenTTL = 7 * 24 * time.Hour
)

var (
	errInvalidAccessToken  = errors.New("invalid access token")
	errInvalidRefreshToken = errors.New("invalid refresh token")
	errRedisNil            = errors.New("redis nil")
)

type authService struct {
	accessTTL  time.Duration
	issuer     string
	keyPrefix  string
	redis      *authRedisClient
	refreshTTL time.Duration
	secret     []byte
}

type authRedisClient struct {
	address  string
	db       int
	password string
	timeout  time.Duration
}

type tokenPair struct {
	AccessExpiresAt  time.Time
	AccessToken      string
	RefreshExpiresAt time.Time
	RefreshToken     string
}

type accessTokenClaims struct {
	ExpiresAt int64  `json:"exp"`
	IssuedAt  int64  `json:"iat"`
	Issuer    string `json:"iss,omitempty"`
	JTI       string `json:"jti"`
	Type      string `json:"typ"`
	UserID    int64  `json:"userId"`
}

func newAuthServiceFromEnv() *authService {
	return &authService{
		accessTTL:  authDurationEnv("KADMIN_JWT_ACCESS_TTL", defaultAccessTokenTTL),
		issuer:     authStringEnv("KADMIN_JWT_ISSUER", "kadmin-vbenapi"),
		keyPrefix:  authStringEnv("KADMIN_AUTH_REDIS_PREFIX", "kadmin:vbenapi:auth"),
		redis:      newAuthRedisClientFromEnv(),
		refreshTTL: authDurationEnv("KADMIN_JWT_REFRESH_TTL", defaultRefreshTokenTTL),
		secret: []byte(authStringEnv(
			"KADMIN_JWT_SECRET",
			"kadmin-vbenapi-dev-secret-change-me",
		)),
	}
}

func newAuthRedisClientFromEnv() *authRedisClient {
	address := strings.TrimSpace(os.Getenv("KADMIN_REDIS_ADDR"))
	if address == "" {
		host := authStringEnv("KADMIN_REDIS_HOST", "127.0.0.1")
		port := authStringEnv("KADMIN_REDIS_PORT", "16379")
		address = fmt.Sprintf("%s:%s", host, port)
	}

	return &authRedisClient{
		address:  address,
		db:       authIntEnv("KADMIN_REDIS_DB", 0),
		password: authStringEnv("KADMIN_REDIS_PASSWORD", "kadmin_redis_pwd"),
		timeout:  2 * time.Second,
	}
}

func (a *authService) issueTokenPair(userID int64) (tokenPair, error) {
	now := time.Now()
	jti, err := randomHex(16)
	if err != nil {
		return tokenPair{}, err
	}

	accessExpiresAt := now.Add(a.accessTTL)
	claims := accessTokenClaims{
		ExpiresAt: accessExpiresAt.Unix(),
		IssuedAt:  now.Unix(),
		Issuer:    a.issuer,
		JTI:       jti,
		Type:      "access",
		UserID:    userID,
	}

	accessToken, err := a.signAccessToken(claims)
	if err != nil {
		return tokenPair{}, err
	}

	refreshToken, err := randomHex(32)
	if err != nil {
		return tokenPair{}, err
	}

	refreshExpiresAt := now.Add(a.refreshTTL)
	if err := a.storeRefreshToken(refreshToken, userID, refreshExpiresAt); err != nil {
		return tokenPair{}, err
	}

	return tokenPair{
		AccessExpiresAt:  accessExpiresAt,
		AccessToken:      accessToken,
		RefreshExpiresAt: refreshExpiresAt,
		RefreshToken:     refreshToken,
	}, nil
}

func (a *authService) parseAccessToken(token string) (accessTokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return accessTokenClaims{}, errInvalidAccessToken
	}

	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return accessTokenClaims{}, errInvalidAccessToken
	}

	var header struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return accessTokenClaims{}, errInvalidAccessToken
	}
	if header.Alg != "HS256" || !strings.EqualFold(header.Typ, "JWT") {
		return accessTokenClaims{}, errInvalidAccessToken
	}

	signingInput := parts[0] + "." + parts[1]
	expectedSignature := signHS256([]byte(signingInput), a.secret)
	actualSignature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return accessTokenClaims{}, errInvalidAccessToken
	}
	if subtle.ConstantTimeCompare(actualSignature, expectedSignature) != 1 {
		return accessTokenClaims{}, errInvalidAccessToken
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return accessTokenClaims{}, errInvalidAccessToken
	}

	var claims accessTokenClaims
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return accessTokenClaims{}, errInvalidAccessToken
	}

	if claims.Type != "access" || claims.UserID <= 0 || claims.JTI == "" {
		return accessTokenClaims{}, errInvalidAccessToken
	}
	if claims.ExpiresAt <= time.Now().Unix() {
		return accessTokenClaims{}, errInvalidAccessToken
	}

	return claims, nil
}

func (a *authService) signAccessToken(claims accessTokenClaims) (string, error) {
	headerJSON := []byte(`{"alg":"HS256","typ":"JWT"}`)
	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	encodedHeader := base64.RawURLEncoding.EncodeToString(headerJSON)
	encodedPayload := base64.RawURLEncoding.EncodeToString(payloadJSON)
	signingInput := encodedHeader + "." + encodedPayload
	signature := base64.RawURLEncoding.EncodeToString(
		signHS256([]byte(signingInput), a.secret),
	)

	return signingInput + "." + signature, nil
}

func (a *authService) storeRefreshToken(
	refreshToken string,
	userID int64,
	expiresAt time.Time,
) error {
	ttl := int(time.Until(expiresAt).Seconds())
	if ttl <= 0 {
		return errInvalidRefreshToken
	}

	_, err := a.redis.do(
		"SET",
		a.refreshTokenKey(refreshToken),
		strconv.FormatInt(userID, 10),
		"EX",
		strconv.Itoa(ttl),
	)
	return err
}

func (a *authService) consumeRefreshToken(refreshToken string) (int64, error) {
	reply, err := a.redis.do("GETDEL", a.refreshTokenKey(refreshToken))
	if err == errRedisNil {
		return 0, errInvalidRefreshToken
	}
	if err != nil {
		return 0, err
	}

	value, ok := reply.(string)
	if !ok {
		return 0, errInvalidRefreshToken
	}

	userID, err := strconv.ParseInt(value, 10, 64)
	if err != nil || userID <= 0 {
		return 0, errInvalidRefreshToken
	}

	return userID, nil
}

func (a *authService) revokeRefreshToken(refreshToken string) error {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil
	}

	_, err := a.redis.do("DEL", a.refreshTokenKey(refreshToken))
	return err
}

func (a *authService) blacklistAccessToken(claims accessTokenClaims) error {
	ttl := int(time.Until(time.Unix(claims.ExpiresAt, 0)).Seconds())
	if claims.JTI == "" || ttl <= 0 {
		return nil
	}

	_, err := a.redis.do(
		"SET",
		a.blacklistKey(claims.JTI),
		"1",
		"EX",
		strconv.Itoa(ttl),
	)
	return err
}

func (a *authService) isAccessTokenBlacklisted(jti string) (bool, error) {
	reply, err := a.redis.do("EXISTS", a.blacklistKey(jti))
	if err != nil {
		return false, err
	}

	exists, ok := reply.(int64)
	if !ok {
		return false, fmt.Errorf("unexpected redis EXISTS reply: %T", reply)
	}
	return exists == 1, nil
}

func (a *authService) refreshTokenKey(refreshToken string) string {
	return fmt.Sprintf("%s:refresh:%s", a.keyPrefix, tokenHash(refreshToken))
}

func (a *authService) blacklistKey(jti string) string {
	return fmt.Sprintf("%s:blacklist:%s", a.keyPrefix, jti)
}

func signHS256(data []byte, secret []byte) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write(data)
	return mac.Sum(nil)
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func randomHex(size int) (string, error) {
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func (r *authRedisClient) do(args ...string) (interface{}, error) {
	conn, err := net.DialTimeout("tcp", r.address, r.timeout)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(r.timeout)); err != nil {
		return nil, err
	}

	reader := bufio.NewReader(conn)
	if r.password != "" {
		if _, err := redisCommand(conn, reader, "AUTH", r.password); err != nil {
			return nil, err
		}
	}
	if r.db > 0 {
		if _, err := redisCommand(conn, reader, "SELECT", strconv.Itoa(r.db)); err != nil {
			return nil, err
		}
	}

	return redisCommand(conn, reader, args...)
}

func redisCommand(
	writer io.Writer,
	reader *bufio.Reader,
	args ...string,
) (interface{}, error) {
	if _, err := fmt.Fprintf(writer, "*%d\r\n", len(args)); err != nil {
		return nil, err
	}
	for _, arg := range args {
		if _, err := fmt.Fprintf(writer, "$%d\r\n%s\r\n", len(arg), arg); err != nil {
			return nil, err
		}
	}
	return readRedisReply(reader)
}

func readRedisReply(reader *bufio.Reader) (interface{}, error) {
	prefix, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}

	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")

	switch prefix {
	case '+':
		return line, nil
	case '-':
		return nil, errors.New("redis: " + line)
	case ':':
		return strconv.ParseInt(line, 10, 64)
	case '$':
		size, err := strconv.Atoi(line)
		if err != nil {
			return nil, err
		}
		if size < 0 {
			return nil, errRedisNil
		}

		data := make([]byte, size+2)
		if _, err := io.ReadFull(reader, data); err != nil {
			return nil, err
		}
		return string(data[:size]), nil
	default:
		return nil, fmt.Errorf("unknown redis reply prefix: %q", prefix)
	}
}

func authStringEnv(key string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func authIntEnv(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func authDurationEnv(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	duration, err := time.ParseDuration(value)
	if err == nil && duration > 0 {
		return duration
	}

	minutes, err := strconv.Atoi(value)
	if err == nil && minutes > 0 {
		return time.Duration(minutes) * time.Minute
	}

	return fallback
}
