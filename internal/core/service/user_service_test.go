package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/poppsfs4za/backend-challenge/internal/adapter/outbound/memory"
	"github.com/poppsfs4za/backend-challenge/internal/core/domain"
	"github.com/poppsfs4za/backend-challenge/internal/core/port"
	"github.com/poppsfs4za/backend-challenge/internal/core/service"
)

// ---------- test double สำหรับ TokenManager ----------

type fakeTokenManager struct {
	token string
	err   error
}

func (f *fakeTokenManager) Generate(userID, email string) (string, time.Time, error) {
	if f.err != nil {
		return "", time.Time{}, f.err
	}
	return f.token, time.Now().Add(time.Hour), nil
}

// newService ประกอบ service ด้วย adapter ปลอมทั้งคู่ — ไม่แตะ Mongo ไม่แตะ JWT
func newService() (*service.UserService, *memory.UserRepository) {
	repo := memory.NewUserRepository()
	svc := service.NewUserService(repo, &fakeTokenManager{token: "fake-token"})
	return svc, repo
}

func mustRegister(t *testing.T, svc *service.UserService, name, email, pass string) *domain.User {
	t.Helper()
	u, err := svc.Register(context.Background(), port.RegisterInput{
		Name: name, Email: email, Password: pass,
	})
	if err != nil {
		t.Fatalf("register %s: unexpected error: %v", email, err)
	}
	return u
}

// ---------- Register ----------

func TestRegister_Success(t *testing.T) {
	svc, _ := newService()

	u := mustRegister(t, svc, "Kraiwit", "kraiwit@mail.com", "secret1234")

	if u.ID == "" {
		t.Error("expected ID to be assigned")
	}
	if u.Password == "secret1234" {
		t.Error("password must be hashed, got plain text")
	}
	if u.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestRegister_NormalizesEmail(t *testing.T) {
	svc, _ := newService()

	u := mustRegister(t, svc, "Kraiwit", "  KRAIWIT@Mail.COM  ", "secret1234")

	if u.Email != "kraiwit@mail.com" {
		t.Errorf("expected normalized email, got %q", u.Email)
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	svc, _ := newService()
	mustRegister(t, svc, "First", "dup@mail.com", "secret1234")

	// สมัครซ้ำด้วยตัวพิมพ์ต่างกัน — ต้องยังถูกปฏิเสธ
	_, err := svc.Register(context.Background(), port.RegisterInput{
		Name: "Second", Email: "DUP@Mail.com", Password: "secret1234",
	})

	if !errors.Is(err, domain.ErrEmailAlreadyExists) {
		t.Errorf("expected ErrEmailAlreadyExists, got %v", err)
	}
}

func TestRegister_TrimsName(t *testing.T) {
	svc, _ := newService()

	u := mustRegister(t, svc, "  Kraiwit  ", "trim@mail.com", "secret1234")

	if u.Name != "Kraiwit" {
		t.Errorf("expected trimmed name, got %q", u.Name)
	}
}

// ---------- Login ----------

func TestLogin_Success(t *testing.T) {
	svc, _ := newService()
	mustRegister(t, svc, "Kraiwit", "login@mail.com", "secret1234")

	res, err := svc.Login(context.Background(), "LOGIN@Mail.com", "secret1234")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Token != "fake-token" {
		t.Errorf("expected token from manager, got %q", res.Token)
	}
	if res.ExpiresAt.Before(time.Now()) {
		t.Error("expected ExpiresAt in the future")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	svc, _ := newService()
	mustRegister(t, svc, "Kraiwit", "wrong@mail.com", "secret1234")

	_, err := svc.Login(context.Background(), "wrong@mail.com", "not-the-password")

	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

// เคสนี้สำคัญเรื่อง security: อีเมลที่ไม่มีในระบบ ต้องได้ error เดียวกับรหัสผ่านผิด
// ไม่ใช่ ErrNotFound ซึ่งจะบอกคนร้ายว่าอีเมลไหนมีอยู่จริง
func TestLogin_UnknownEmail_LooksLikeWrongPassword(t *testing.T) {
	svc, _ := newService()

	_, err := svc.Login(context.Background(), "nobody@mail.com", "secret1234")

	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
	if errors.Is(err, domain.ErrNotFound) {
		t.Error("must not leak ErrNotFound — enables user enumeration")
	}
}

func TestLogin_TokenGenerationFails(t *testing.T) {
	repo := memory.NewUserRepository()
	boom := errors.New("token service down")
	svc := service.NewUserService(repo, &fakeTokenManager{err: boom})

	mustRegister(t, svc, "Kraiwit", "tok@mail.com", "secret1234")

	_, err := svc.Login(context.Background(), "tok@mail.com", "secret1234")
	if !errors.Is(err, boom) {
		t.Errorf("expected token error to propagate, got %v", err)
	}
}

// ---------- CRUD ----------

func TestGetByID_NotFound(t *testing.T) {
	svc, _ := newService()

	_, err := svc.GetByID(context.Background(), "does-not-exist")

	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestList_ReturnsAll(t *testing.T) {
	svc, _ := newService()
	mustRegister(t, svc, "A", "a@mail.com", "secret1234")
	mustRegister(t, svc, "B", "b@mail.com", "secret1234")

	users, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(users) != 2 {
		t.Errorf("expected 2 users, got %d", len(users))
	}
}

func TestUpdate_NameOnly(t *testing.T) {
	svc, _ := newService()
	u := mustRegister(t, svc, "Old Name", "upd@mail.com", "secret1234")

	updated, err := svc.Update(context.Background(), u.ID, "New Name", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updated.Name != "New Name" {
		t.Errorf("expected name updated, got %q", updated.Name)
	}
	if updated.Email != "upd@mail.com" {
		t.Errorf("email must not change, got %q", updated.Email)
	}
}

func TestUpdate_EmailTakenByAnotherUser(t *testing.T) {
	svc, _ := newService()
	mustRegister(t, svc, "A", "taken@mail.com", "secret1234")
	b := mustRegister(t, svc, "B", "free@mail.com", "secret1234")

	_, err := svc.Update(context.Background(), b.ID, "", "taken@mail.com")

	if !errors.Is(err, domain.ErrEmailAlreadyExists) {
		t.Errorf("expected ErrEmailAlreadyExists, got %v", err)
	}
}

// เคสขอบ: เปลี่ยนอีเมลเป็นค่าเดิมของตัวเอง ต้องไม่ถูกมองว่าซ้ำ
func TestUpdate_SameEmailAsSelf(t *testing.T) {
	svc, _ := newService()
	u := mustRegister(t, svc, "Self", "self@mail.com", "secret1234")

	_, err := svc.Update(context.Background(), u.ID, "Renamed", "self@mail.com")
	if err != nil {
		t.Fatalf("updating with own email should succeed, got %v", err)
	}
}

func TestDelete(t *testing.T) {
	svc, _ := newService()
	u := mustRegister(t, svc, "Bye", "del@mail.com", "secret1234")

	if err := svc.Delete(context.Background(), u.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := svc.GetByID(context.Background(), u.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestDelete_NotFound(t *testing.T) {
	svc, _ := newService()

	err := svc.Delete(context.Background(), "ghost")

	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestCount(t *testing.T) {
	svc, _ := newService()
	mustRegister(t, svc, "A", "c1@mail.com", "secret1234")
	mustRegister(t, svc, "B", "c2@mail.com", "secret1234")

	n, err := svc.Count(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 2 {
		t.Errorf("expected 2, got %d", n)
	}
}
