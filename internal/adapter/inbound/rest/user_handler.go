package rest

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/poppbsfs4za/backend-challenge-7solutions/internal/core/domain"
	"github.com/poppbsfs4za/backend-challenge-7solutions/internal/core/port"
)

type UserHandler struct {
	svc port.UserService // ขึ้นกับ inbound port ไม่ใช่ struct ตัวจริง
}

func NewUserHandler(svc port.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

// POST /api/v1/auth/register
func (h *UserHandler) Register(c echo.Context) error {
	var req RegisterRequest
	if err := bindAndValidate(c, &req); err != nil {
		return err
	}

	u, err := h.svc.Register(c.Request().Context(), port.RegisterInput{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusCreated, toUserResponse(u))
}

// POST /api/v1/auth/login
func (h *UserHandler) Login(c echo.Context) error {
	var req LoginRequest
	if err := bindAndValidate(c, &req); err != nil {
		return err
	}

	res, err := h.svc.Login(c.Request().Context(), req.Email, req.Password)
	if err != nil {
		return mapError(err)
	}

	return c.JSON(http.StatusOK, AuthResponse{
		AccessToken: res.Token,
		TokenType:   "Bearer",
		ExpiresAt:   res.ExpiresAt,
		User:        toUserResponse(res.User),
	})
}

// POST /api/v1/users
func (h *UserHandler) Create(c echo.Context) error {
	var req RegisterRequest
	if err := bindAndValidate(c, &req); err != nil {
		return err
	}

	u, err := h.svc.Create(c.Request().Context(), port.RegisterInput{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusCreated, toUserResponse(u))
}

// GET /api/v1/users
func (h *UserHandler) List(c echo.Context) error {
	users, err := h.svc.List(c.Request().Context())
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, ListUsersResponse{
		Data:  toUserResponses(users),
		Total: len(users),
	})
}

// GET /api/v1/users/:id
func (h *UserHandler) GetByID(c echo.Context) error {
	u, err := h.svc.GetByID(c.Request().Context(), c.Param("id"))
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, toUserResponse(u))
}

// PUT /api/v1/users/:id
func (h *UserHandler) Update(c echo.Context) error {
	var req UpdateUserRequest
	if err := bindAndValidate(c, &req); err != nil {
		return err
	}
	if req.Name == "" && req.Email == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "at least one of name or email must be provided")
	}

	u, err := h.svc.Update(c.Request().Context(), c.Param("id"), req.Name, req.Email)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, toUserResponse(u))
}

// DELETE /api/v1/users/:id
func (h *UserHandler) Delete(c echo.Context) error {
	if err := h.svc.Delete(c.Request().Context(), c.Param("id")); err != nil {
		return mapError(err)
	}
	return c.NoContent(http.StatusNoContent)
}

func bindAndValidate(c echo.Context, req any) error {
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := c.Validate(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return nil
}

// mapError แปลง domain error เป็น HTTP status
// รวมไว้ที่เดียว handler แต่ละตัวจะได้ไม่ต้องเขียน if ซ้ำ
func mapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrEmailAlreadyExists):
		return echo.NewHTTPError(http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrInvalidCredentials):
		return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
	case errors.Is(err, domain.ErrInvalidID):
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	default:
		// ไม่ส่ง error จริงจาก DB กลับไปให้ client เห็น
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}
}
