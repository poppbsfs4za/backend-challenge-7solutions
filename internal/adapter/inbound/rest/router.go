package rest

import (
	"github.com/labstack/echo/v4"

	jwtpkg "github.com/poppbsfs4za/backend-challenge-7solutions/pkg/jwt"
	mw "github.com/poppbsfs4za/backend-challenge-7solutions/pkg/middleware"
)

func RegisterRoutes(e *echo.Echo, h *UserHandler, jwtManager *jwtpkg.Manager) {
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(200, echo.Map{"status": "ok"})
	})

	api := e.Group("/api/v1")

	// --- Public: ไม่ต้องมี token ---
	auth := api.Group("/auth")
	auth.POST("/register", h.Register)
	auth.POST("/login", h.Login)

	// --- Protected: ต้องแนบ Bearer token ---
	users := api.Group("/users", mw.JWTAuth(jwtManager))
	users.POST("", h.Create)
	users.GET("", h.List)
	users.GET("/:id", h.GetByID)
	users.PUT("/:id", h.Update)
	users.DELETE("/:id", h.Delete)
}
