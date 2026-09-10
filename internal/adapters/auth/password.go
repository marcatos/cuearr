package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const SessionCookieName = "cuearr_session"

const bcryptCost = bcrypt.DefaultCost

// HashPassword returns a bcrypt hash of plain.
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(b), nil
}

// CheckPassword compares plain against a bcrypt hash.
func CheckPassword(hash, plain string) bool {
	if hash == "" || plain == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// NewSessionCookie creates a signed HttpOnly session cookie.
func NewSessionCookie(secret []byte, ttl time.Duration, secure ...bool) (*http.Cookie, error) {
	if len(secret) == 0 {
		return nil, fmt.Errorf("session secret required")
	}
	exp := time.Now().Add(ttl).Unix()
	token, err := signSession(secret, exp)
	if err != nil {
		return nil, err
	}
	maxAge := int(ttl.Seconds())
	if maxAge < 0 {
		maxAge = 0
	}
	cookie := &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	}
	if len(secure) > 0 {
		cookie.Secure = secure[0]
	}
	return cookie, nil
}

// ClearSessionCookie returns a cookie that removes the HttpOnly session in the browser.
func ClearSessionCookie(secure ...bool) *http.Cookie {
	cookie := &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	}
	if len(secure) > 0 {
		cookie.Secure = secure[0]
	}
	return cookie
}

// ValidSessionCookie reports whether c is a valid signed session for secret.
func ValidSessionCookie(secret []byte, c *http.Cookie) bool {
	if c == nil || c.Name != SessionCookieName || c.Value == "" || len(secret) == 0 {
		return false
	}
	exp, ok := verifySession(secret, c.Value)
	if !ok {
		return false
	}
	return time.Now().Unix() <= exp
}

func signSession(secret []byte, expUnix int64) (string, error) {
	payload := strconv.FormatInt(expUnix, 10)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "." + sig, nil
}

func verifySession(secret []byte, token string) (expUnix int64, ok bool) {
	payload, sig, found := strings.Cut(token, ".")
	if !found || payload == "" || sig == "" {
		return 0, false
	}
	exp, err := strconv.ParseInt(payload, 10, 64)
	if err != nil {
		return 0, false
	}
	expected, err := signSession(secret, exp)
	if err != nil {
		return 0, false
	}
	_, expectedSig, _ := strings.Cut(expected, ".")
	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return 0, false
	}
	return exp, true
}
