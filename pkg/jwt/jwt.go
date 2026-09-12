package jwt

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/poppsfs4za/backend-challenge/internal/core/port"
)

var ErrInvalidToken = errors.New("invalid or expired token")

type Claims struct {
	UserID string `json:"uid"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

type Manager struct {
	secret []byte
	expire time.Duration
}

// ยืนยันตอน compile ว่า implement outbound port ครบ
var _ port.TokenManager = (*Manager)(nil)

func NewManager(secret string, expire time.Duration) *Manager {
	return &Manager{secret: []byte(secret), expire: expire}
}

func (m *Manager) Generate(userID, email string) (string, time.Time, error) {
	expiresAt := time.Now().Add(m.expire)

	claims := Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    "backend-challenge",
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	// HS256 ตามที่โจทย์ระบุ
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, expiresAt, nil
}

// Verify ไม่ได้อยู่ใน port เพราะเป็นงานของ middleware ฝั่ง inbound เท่านั้น
// service ไม่เคยต้องตรวจ token เอง
func (m *Manager) Verify(tokenStr string) (*Claims, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return m.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))

	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
