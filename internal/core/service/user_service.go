package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/poppbsfs4za/backend-challenge-7solutions/internal/core/domain"
	"github.com/poppbsfs4za/backend-challenge-7solutions/internal/core/port"
	"github.com/poppbsfs4za/backend-challenge-7solutions/pkg/hash"
)

type UserService struct {
	repo  port.UserRepository
	token port.TokenManager
}

// ยืนยันตอน compile ว่า implement inbound port ครบ
var _ port.UserService = (*UserService)(nil)

func NewUserService(repo port.UserRepository, token port.TokenManager) *UserService {
	return &UserService{repo: repo, token: token}
}

func (s *UserService) Register(ctx context.Context, in port.RegisterInput) (*domain.User, error) {
	email := normalizeEmail(in.Email)

	// กฎธุรกิจ: อีเมลห้ามซ้ำ
	_, err := s.repo.GetByEmail(ctx, email)
	switch {
	case err == nil:
		return nil, domain.ErrEmailAlreadyExists
	case !errors.Is(err, domain.ErrNotFound):
		return nil, err // error จริงจาก DB ไม่ใช่แค่ "ไม่เจอ"
	}

	// กฎธุรกิจ: รหัสผ่านต้อง hash ก่อนลงฐานข้อมูลเสมอ
	hashed, err := hash.HashPassword(in.Password)
	if err != nil {
		return nil, err
	}

	u := &domain.User{
		Name:      strings.TrimSpace(in.Name),
		Email:     email,
		Password:  hashed,
		CreatedAt: time.Now().UTC(),
	}

	if err := s.repo.Create(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

func (s *UserService) Login(ctx context.Context, email, password string) (*port.AuthResult, error) {
	u, err := s.repo.GetByEmail(ctx, normalizeEmail(email))
	if err != nil {
		// กลืน ErrNotFound เป็น ErrInvalidCredentials โดยตั้งใจ
		// ไม่ให้คนร้ายรู้ว่าอีเมลไหนมีอยู่จริง
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrInvalidCredentials
		}
		return nil, err
	}

	if !hash.ComparePassword(u.Password, password) {
		return nil, domain.ErrInvalidCredentials
	}

	tk, expiresAt, err := s.token.Generate(u.ID, u.Email)
	if err != nil {
		return nil, err
	}

	return &port.AuthResult{Token: tk, ExpiresAt: expiresAt, User: u}, nil
}

// Create คือโจทย์ข้อ 3 "Create a new user" — flow เดียวกับ Register
func (s *UserService) Create(ctx context.Context, in port.RegisterInput) (*domain.User, error) {
	return s.Register(ctx, in)
}

func (s *UserService) GetByID(ctx context.Context, id string) (*domain.User, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *UserService) List(ctx context.Context) ([]*domain.User, error) {
	return s.repo.List(ctx)
}

func (s *UserService) Update(ctx context.Context, id, name, email string) (*domain.User, error) {
	name = strings.TrimSpace(name)

	if email != "" {
		email = normalizeEmail(email)

		// อีเมลใหม่ต้องไม่ชนคนอื่น (ชนตัวเองไม่นับ)
		existing, err := s.repo.GetByEmail(ctx, email)
		switch {
		case err == nil && existing.ID != id:
			return nil, domain.ErrEmailAlreadyExists
		case err != nil && !errors.Is(err, domain.ErrNotFound):
			return nil, err
		}
	}

	return s.repo.Update(ctx, id, name, email)
}

func (s *UserService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *UserService) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}

func normalizeEmail(e string) string {
	return strings.ToLower(strings.TrimSpace(e))
}
