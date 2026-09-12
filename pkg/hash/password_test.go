package hash_test

import (
	"strings"
	"testing"

	"github.com/poppbsfs4za/backend-challenge-7solutions/pkg/hash"
)

func TestHashPassword(t *testing.T) {
	hashed, err := hash.HashPassword("secret1234")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if hashed == "secret1234" {
		t.Error("hash must differ from plain text")
	}
	if !strings.HasPrefix(hashed, "$2a$") {
		t.Errorf("expected bcrypt format, got %q", hashed)
	}
}

// bcrypt ใส่ salt สุ่มทุกครั้ง — hash เดียวกันสองรอบต้องได้คนละค่า
func TestHashPassword_SaltIsRandom(t *testing.T) {
	a, _ := hash.HashPassword("secret1234")
	b, _ := hash.HashPassword("secret1234")

	if a == b {
		t.Error("expected different hashes due to random salt")
	}
}

func TestComparePassword(t *testing.T) {
	hashed, _ := hash.HashPassword("secret1234")

	if !hash.ComparePassword(hashed, "secret1234") {
		t.Error("correct password should match")
	}
	if hash.ComparePassword(hashed, "wrong-password") {
		t.Error("wrong password must not match")
	}
	if hash.ComparePassword(hashed, "Secret1234") {
		t.Error("password comparison must be case-sensitive")
	}
}

// ยืนยันข้อจำกัด 72 bytes ของ bcrypt ที่เป็นเหตุผลของ max=72 ใน DTO
func TestComparePassword_BcryptTruncatesAt72Bytes(t *testing.T) {
	long := strings.Repeat("a", 72)
	hashed, _ := hash.HashPassword(long)

	if !hash.ComparePassword(hashed, long+"extra-ignored-characters") {
		t.Skip("driver rejects >72 bytes instead of truncating — also acceptable")
	}
	t.Log("confirmed: bcrypt ignores bytes beyond 72 — hence max=72 validation")
}
