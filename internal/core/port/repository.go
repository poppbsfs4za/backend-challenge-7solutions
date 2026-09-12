package port

import (
	"context"

	"github.com/poppbsfs4za/backend-challenge-7solutions/internal/core/domain"
)

// UserRepository คือสิ่งที่แกนกลาง "ต้องการ" จากที่เก็บข้อมูล
// ใครจะ implement ก็ได้ — Mongo, memory, Postgres
type UserRepository interface {
	Create(ctx context.Context, u *domain.User) error
	GetByID(ctx context.Context, id string) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	List(ctx context.Context) ([]*domain.User, error)
	Update(ctx context.Context, id, name, email string) (*domain.User, error)
	Delete(ctx context.Context, id string) error
	Count(ctx context.Context) (int64, error)
}
