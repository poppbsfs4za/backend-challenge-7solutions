package jwt_test

import (
	"strings"
	"testing"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"

	jwtpkg "github.com/poppbsfs4za/backend-challenge-7solutions/pkg/jwt"
)

const secret = "test-secret"

func TestGenerateAndVerify(t *testing.T) {
	m := jwtpkg.NewManager(secret, time.Hour)

	token, expiresAt, err := m.Generate("user-123", "a@mail.com")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if expiresAt.Before(time.Now()) {
		t.Error("expected future expiry")
	}

	claims, err := m.Verify(token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.UserID != "user-123" {
		t.Errorf("expected user-123, got %q", claims.UserID)
	}
	if claims.Email != "a@mail.com" {
		t.Errorf("expected a@mail.com, got %q", claims.Email)
	}
}

func TestVerify_UsesHS256(t *testing.T) {
	m := jwtpkg.NewManager(secret, time.Hour)
	token, _, _ := m.Generate("u", "a@mail.com")

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 JWT segments, got %d", len(parts))
	}

	parsed, _, err := jwtlib.NewParser().ParseUnverified(token, jwtlib.MapClaims{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.Method.Alg() != "HS256" {
		t.Errorf("expected HS256 as required by the spec, got %s", parsed.Method.Alg())
	}
}

func TestVerify_WrongSecret(t *testing.T) {
	token, _, _ := jwtpkg.NewManager(secret, time.Hour).Generate("u", "a@mail.com")

	_, err := jwtpkg.NewManager("different-secret", time.Hour).Verify(token)
	if err == nil {
		t.Error("expected verification to fail with a different secret")
	}
}

func TestVerify_Expired(t *testing.T) {
	m := jwtpkg.NewManager(secret, -time.Hour) // หมดอายุไปแล้ว

	token, _, _ := m.Generate("u", "a@mail.com")

	if _, err := m.Verify(token); err == nil {
		t.Error("expected expired token to be rejected")
	}
}

// ★ เคสสำคัญที่สุด: ป้องกัน algorithm confusion attack
// คนร้ายสร้าง token ด้วย alg=none แล้วตัดลายเซ็นทิ้ง
func TestVerify_RejectsNoneAlgorithm(t *testing.T) {
	unsigned := jwtlib.NewWithClaims(jwtlib.SigningMethodNone, jwtlib.MapClaims{
		"uid":   "attacker",
		"email": "evil@mail.com",
		"exp":   time.Now().Add(time.Hour).Unix(),
	})
	token, err := unsigned.SignedString(jwtlib.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("craft token: %v", err)
	}

	if _, err := jwtpkg.NewManager(secret, time.Hour).Verify(token); err == nil {
		t.Error("SECURITY: accepted a token signed with alg=none")
	}
}

func TestVerify_GarbageInput(t *testing.T) {
	m := jwtpkg.NewManager(secret, time.Hour)

	for _, bad := range []string{"", "not-a-token", "a.b.c", "....."} {
		if _, err := m.Verify(bad); err == nil {
			t.Errorf("expected rejection for %q", bad)
		}
	}
}
