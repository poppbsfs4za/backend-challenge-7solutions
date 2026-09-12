package memory_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/poppbsfs4za/backend-challenge-7solutions/internal/adapter/outbound/memory"
	"github.com/poppbsfs4za/backend-challenge-7solutions/internal/core/domain"
)

func newUser(name, email string) *domain.User {
	return &domain.User{
		Name:      name,
		Email:     email,
		Password:  "hashed",
		CreatedAt: time.Now().UTC(),
	}
}

func TestCreate_AssignsID(t *testing.T) {
	repo := memory.NewUserRepository()
	u := newUser("A", "a@mail.com")

	if err := repo.Create(context.Background(), u); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.ID == "" {
		t.Error("expected ID to be assigned")
	}
}

func TestCreate_DuplicateEmail(t *testing.T) {
	repo := memory.NewUserRepository()
	ctx := context.Background()

	if err := repo.Create(ctx, newUser("A", "dup@mail.com")); err != nil {
		t.Fatalf("first create: %v", err)
	}

	err := repo.Create(ctx, newUser("B", "dup@mail.com"))
	if !errors.Is(err, domain.ErrEmailAlreadyExists) {
		t.Errorf("expected ErrEmailAlreadyExists, got %v", err)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	repo := memory.NewUserRepository()

	_, err := repo.GetByID(context.Background(), "nope")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetByEmail(t *testing.T) {
	repo := memory.NewUserRepository()
	ctx := context.Background()
	u := newUser("A", "find@mail.com")
	_ = repo.Create(ctx, u)

	got, err := repo.GetByEmail(ctx, "find@mail.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != u.ID {
		t.Errorf("expected id %q, got %q", u.ID, got.ID)
	}
}

// พิสูจน์ว่า repo คืน copy ไม่ใช่ pointer เข้าไปใน store
// ถ้าพังข้อนี้ คนเรียกจะแก้ข้อมูลใน store ได้โดยไม่ผ่าน mutex
func TestGetByID_ReturnsCopy(t *testing.T) {
	repo := memory.NewUserRepository()
	ctx := context.Background()
	u := newUser("Original", "copy@mail.com")
	_ = repo.Create(ctx, u)

	got, _ := repo.GetByID(ctx, u.ID)
	got.Name = "Mutated"

	again, _ := repo.GetByID(ctx, u.ID)
	if again.Name != "Original" {
		t.Errorf("store was mutated through returned pointer: %q", again.Name)
	}
}

func TestList_SortedNewestFirst(t *testing.T) {
	repo := memory.NewUserRepository()
	ctx := context.Background()

	older := newUser("Older", "old@mail.com")
	older.CreatedAt = time.Now().UTC().Add(-time.Hour)
	_ = repo.Create(ctx, older)

	newer := newUser("Newer", "new@mail.com")
	_ = repo.Create(ctx, newer)

	users, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
	if users[0].Name != "Newer" {
		t.Errorf("expected newest first, got %q", users[0].Name)
	}
}

func TestUpdate_PartialFields(t *testing.T) {
	repo := memory.NewUserRepository()
	ctx := context.Background()
	u := newUser("Old", "upd@mail.com")
	_ = repo.Create(ctx, u)

	got, err := repo.Update(ctx, u.ID, "New", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "New" {
		t.Errorf("expected name updated, got %q", got.Name)
	}
	if got.Email != "upd@mail.com" {
		t.Errorf("email must not change, got %q", got.Email)
	}
}

func TestUpdate_NotFound(t *testing.T) {
	repo := memory.NewUserRepository()

	_, err := repo.Update(context.Background(), "ghost", "X", "")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDelete(t *testing.T) {
	repo := memory.NewUserRepository()
	ctx := context.Background()
	u := newUser("Bye", "del@mail.com")
	_ = repo.Create(ctx, u)

	if err := repo.Delete(ctx, u.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := repo.GetByID(ctx, u.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestCount(t *testing.T) {
	repo := memory.NewUserRepository()
	ctx := context.Background()
	_ = repo.Create(ctx, newUser("A", "c1@mail.com"))
	_ = repo.Create(ctx, newUser("B", "c2@mail.com"))

	n, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 2 {
		t.Errorf("expected 2, got %d", n)
	}
}

// ทดสอบว่า RWMutex ทำงานจริง — รันด้วย -race จะจับ data race ได้ถ้าลืมล็อก
func TestConcurrentAccess(t *testing.T) {
	repo := memory.NewUserRepository()
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)

		go func(n int) {
			defer wg.Done()
			_ = repo.Create(ctx, newUser("U", string(rune('a'+n%26))+"@mail.com"))
		}(i)

		go func() {
			defer wg.Done()
			_, _ = repo.Count(ctx)
			_, _ = repo.List(ctx)
		}()
	}
	wg.Wait()
}
