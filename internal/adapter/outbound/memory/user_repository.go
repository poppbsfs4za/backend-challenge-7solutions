package memory

import (
	"context"
	"sort"
	"strconv"
	"sync"

	"github.com/poppsfs4za/backend-challenge/internal/core/domain"
	"github.com/poppsfs4za/backend-challenge/internal/core/port"
)

type UserRepository struct {
	mu     sync.RWMutex
	users  map[string]*domain.User
	nextID int64
}

var _ port.UserRepository = (*UserRepository)(nil)

func NewUserRepository() *UserRepository {
	return &UserRepository{users: make(map[string]*domain.User)}
}

func (r *UserRepository) Create(ctx context.Context, u *domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// จำลอง unique index บน email
	for _, existing := range r.users {
		if existing.Email == u.Email {
			return domain.ErrEmailAlreadyExists
		}
	}

	r.nextID++
	u.ID = strconv.FormatInt(r.nextID, 10)

	// เก็บสำเนา ไม่ใช่ pointer ตัวเดิม — กันคนนอกแก้ข้อมูลใน store ผ่าน pointer
	cp := *u
	r.users[u.ID] = &cp
	return nil
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	u, ok := r.users[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *u
	return &cp, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, u := range r.users {
		if u.Email == email {
			cp := *u
			return &cp, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *UserRepository) List(ctx context.Context) ([]*domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]*domain.User, 0, len(r.users))
	for _, u := range r.users {
		cp := *u
		out = append(out, &cp)
	}

	// เรียงใหม่สุดขึ้นก่อน ให้พฤติกรรมตรงกับ mongo adapter
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

func (r *UserRepository) Update(ctx context.Context, id, name, email string) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	u, ok := r.users[id]
	if !ok {
		return nil, domain.ErrNotFound
	}

	if email != "" {
		for otherID, other := range r.users {
			if other.Email == email && otherID != id {
				return nil, domain.ErrEmailAlreadyExists
			}
		}
		u.Email = email
	}
	if name != "" {
		u.Name = name
	}

	cp := *u
	return &cp, nil
}

func (r *UserRepository) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.users[id]; !ok {
		return domain.ErrNotFound
	}
	delete(r.users, id)
	return nil
}

func (r *UserRepository) Count(ctx context.Context) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return int64(len(r.users)), nil
}
