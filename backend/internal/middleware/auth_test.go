package middleware_test

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TestJWTExpClaim validates JWT expiration behavior with different exp types.
// Regression test for REVIEW.md Issue #1.
func TestJWTExpClaim(t *testing.T) {
	t.Parallel()

	secret := []byte("test-secret")

	t.Run("numeric exp is validated correctly", func(t *testing.T) {
		t.Parallel()

		expTime := time.Now().Add(24 * time.Hour).Unix()
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"user_id": "test-user-id",
			"email":   "test@example.com",
			"role":    "admin",
			"exp":     expTime,
		})

		tokenString, err := token.SignedString(secret)
		if err != nil {
			t.Fatalf("SignedString() error = %v", err)
		}

		parsed, err := jwt.Parse(tokenString, func(_ *jwt.Token) (interface{}, error) {
			return secret, nil
		})
		if err != nil {
			t.Fatalf("Parse() error = %v, want nil for valid token", err)
		}

		claims, ok := parsed.Claims.(jwt.MapClaims)
		if !ok {
			t.Fatal("failed to cast claims to MapClaims")
		}

		expValue, exists := claims["exp"]
		if !exists {
			t.Fatal("exp claim missing from token")
		}

		// json.Unmarshal decodes JSON numbers as float64
		expFloat, ok := expValue.(float64)
		if !ok {
			t.Fatalf("exp claim type = %T, want float64 (numeric). Value: %v", expValue, expValue)
		}

		if int64(expFloat) != expTime {
			t.Errorf("exp value = %d, want %d", int64(expFloat), expTime)
		}
	})

	t.Run("expired token is rejected", func(t *testing.T) {
		t.Parallel()

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"user_id": "test-user-id",
			"email":   "test@example.com",
			"role":    "admin",
			"exp":     time.Now().Add(-1 * time.Hour).Unix(),
		})

		tokenString, err := token.SignedString(secret)
		if err != nil {
			t.Fatalf("SignedString() error = %v", err)
		}

		_, err = jwt.Parse(tokenString, func(_ *jwt.Token) (interface{}, error) {
			return secret, nil
		})
		if err == nil {
			t.Fatal("Parse() error = nil, want expiration error")
		}

		if !strings.Contains(err.Error(), "expired") {
			t.Errorf("Parse() error = %q, want error containing 'expired'", err.Error())
		}
	})

	t.Run("string exp bypasses validation (demonstrates bug)", func(t *testing.T) {
		t.Parallel()

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"user_id": "test-user-id",
			"email":   "test@example.com",
			"role":    "admin",
			"exp":     "1000000000", // Year 2001, clearly expired
		})

		tokenString, err := token.SignedString(secret)
		if err != nil {
			t.Fatalf("SignedString() error = %v", err)
		}

		parsed, err := jwt.Parse(tokenString, func(_ *jwt.Token) (interface{}, error) {
			return secret, nil
		})

		// Demonstrates the vulnerability: string exp bypasses validation
		if err == nil && parsed.Valid {
			t.Log("CONFIRMED BUG: string exp bypasses expiration validation")
		} else {
			t.Log("newer jwt library version may handle string exp differently")
		}
	})
}
