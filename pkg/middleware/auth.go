package middleware

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	jwtpkg "github.com/poppbsfs4za/backend-challenge-7solutions/pkg/jwt"
)

const (
	ContextUserID = "user_id"
	ContextEmail  = "user_email"
)

// JWTAuth ป้องกัน endpoint ตามโจทย์ข้อ 2
func JWTAuth(m *jwtpkg.Manager) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			header := c.Request().Header.Get("Authorization")
			if header == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "missing authorization header")
			}

			parts := strings.SplitN(header, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				return echo.NewHTTPError(http.StatusUnauthorized, "authorization header must be 'Bearer <token>'")
			}

			claims, err := m.Verify(strings.TrimSpace(parts[1]))
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid or expired token")
			}

			// ฝากข้อมูลผู้ใช้ไว้ให้ handler ปลายทางหยิบไปใช้ได้
			c.Set(ContextUserID, claims.UserID)
			c.Set(ContextEmail, claims.Email)

			return next(c)
		}
	}
}
