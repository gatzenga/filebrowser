package fbhttp

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	fbAuth "github.com/filebrowser/filebrowser/v2/auth"
	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/users"
)

// Regression for GHSA-v3jv-rmh2-635j: under proxy auth with a non-default
// logout page the JWT expiration is waived, because the proxy owns the session
// lifetime. That exception used to apply to every route on the strength of the
// token alone, so a token stolen before it expired kept working — and could be
// renewed — indefinitely. The proxy must still assert the same identity.
func TestExpiredTokenNeedsProxyAssertion(t *testing.T) {
	const proxyHeader = "X-Fb-User"

	key := []byte("test-signing-key")
	perm := users.Permissions{Download: true}
	st := scopedUserStorage(t, t.TempDir(), perm, key)

	if err := st.Settings.Save(&settings.Settings{
		Key:        key,
		AuthMethod: fbAuth.MethodProxyAuth,
		LogoutPage: "/logged-out",
	}); err != nil {
		t.Fatalf("failed to save settings: %v", err)
	}
	if err := st.Auth.Save(&fbAuth.ProxyAuth{Header: proxyHeader}); err != nil {
		t.Fatalf("failed to save auther: %v", err)
	}

	expired := &authToken{
		User: userInfo{ID: 1, Username: "u", Perm: perm},
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
	}
	expiredToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, expired).SignedString(key)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	protected := withUser(func(w http.ResponseWriter, _ *http.Request, _ *data) (int, error) {
		_, writeErr := w.Write([]byte("protected"))
		return 0, writeErr
	})

	get := func(token, proxyUser string) *httptest.ResponseRecorder {
		req, _ := http.NewRequest(http.MethodGet, "/", http.NoBody)
		req.Header.Set("X-Auth", token)
		if proxyUser != "" {
			req.Header.Set(proxyHeader, proxyUser)
		}
		rec := httptest.NewRecorder()
		handle(protected, "", st, &settings.Server{}).ServeHTTP(rec, req)
		return rec
	}

	t.Run("expired token alone is rejected", func(t *testing.T) {
		if rec := get(expiredToken, ""); rec.Code != http.StatusUnauthorized {
			t.Errorf("VULNERABLE: expired token without the proxy header = %d, body=%q; want 401", rec.Code, rec.Body.String())
		}
	})

	t.Run("expired token for another identity is rejected", func(t *testing.T) {
		if rec := get(expiredToken, "someone-else"); rec.Code != http.StatusUnauthorized {
			t.Errorf("VULNERABLE: expired token with a foreign proxy identity = %d; want 401", rec.Code)
		}
	})

	t.Run("expired token the proxy still asserts is accepted", func(t *testing.T) {
		if rec := get(expiredToken, "u"); rec.Code != http.StatusOK {
			t.Errorf("expired token asserted by the proxy = %d, body=%q; want 200", rec.Code, rec.Body.String())
		}
	})

	t.Run("valid token needs no assertion", func(t *testing.T) {
		if rec := get(signToken(t, perm, key), ""); rec.Code != http.StatusOK {
			t.Errorf("valid token = %d, body=%q; want 200", rec.Code, rec.Body.String())
		}
	})
}
