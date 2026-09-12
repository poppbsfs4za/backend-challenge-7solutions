package port

import (
	"context"
	"time"

	"github.com/poppbsfs4za/backend-challenge-7solutions/internal/core/domain"
)

type RegisterInput struct {
	Name     string
	Email    string
	Password string
}

type AuthResult struct {
	Token     string
	ExpiresAt time.Time
	User      *domain.User
}

// UserService คือสิ่งที่โลกภายนอก "สั่งได้" กับแกนกลาง
// REST adapter ขึ้นกับ interface นี้ ไม่ใช่ struct ตัวจริง
type UserService interface {
	Register(ctx context.Context, in RegisterInput) (*domain.User, error)
	Login(ctx context.Context, email, password string) (*AuthResult, error)

	Create(ctx context.Context, in RegisterInput) (*domain.User, error)
	GetByID(ctx context.Context, id string) (*domain.User, error)
	List(ctx context.Context) ([]*domain.User, error)
	Update(ctx context.Context, id, name, email string) (*domain.User, error)
	Delete(ctx context.Context, id string) error
	Count(ctx context.Context) (int64, error)
}
