package kadmin

import (
	"errors"
	"testing"
	"time"
)

func TestAccessTokenRejectsWrongIssuer(t *testing.T) {
	auth := &authService{issuer: "expected-service", secret: []byte("issuer-test-secret")}
	for _, issuer := range []string{"expected-service", "different-service", ""} {
		t.Run(issuer, func(t *testing.T) {
			token, err := auth.signAccessToken(accessTokenClaims{
				Issuer: issuer, Type: "access", UserID: 1, JTI: "issuer-test",
				IssuedAt: time.Now().Unix(), ExpiresAt: time.Now().Add(time.Minute).Unix(),
			})
			if err != nil {
				t.Fatal(err)
			}
			_, err = auth.parseAccessToken(token)
			if issuer == auth.issuer {
				if err != nil {
					t.Fatalf("issued token rejected: %v", err)
				}
			} else if !errors.Is(err, errInvalidAccessToken) {
				t.Fatalf("token from issuer %q accepted; want invalid access token, got %v", issuer, err)
			}
		})
	}
}
