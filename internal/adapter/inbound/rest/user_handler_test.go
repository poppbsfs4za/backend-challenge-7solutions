package rest_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/poppsfs4za/backend-challenge/internal/adapter/inbound/rest"
	"github.com/poppsfs4za/backend-challenge/internal/core/domain"
	"github.com/poppsfs4za/backend-challenge/internal/core/port"
	"github.com/poppsfs4za/backend-challenge/pkg/validator"
)

var errUnexpected = errors.New("mongo: connection string invalid")

// ---------- service ปลอม: คุมผลลัพธ์ได้ทุกเคส ----------

type stubService struct {
	user *domain.User
	auth *port.AuthResult
	list []*domain.User
	err  error
}

var _ port.UserService = (*stubService)(nil)

func (s *stubService) Register(ctx context.Context, in port.RegisterInput) (*domain.User, error) {
	return s.user, s.err
}
func (s *stubService) Create(ctx context.Context, in port.RegisterInput) (*domain.User, error) {
	return s.user, s.err
}
func (s *stubService) Login(ctx context.Context, email, password string) (*port.AuthResult, error) {
	return s.auth, s.err
}
func (s *stubService) GetByID(ctx context.Context, id string) (*domain.User, error) {
	return s.user, s.err
}
func (s *stubService) List(ctx context.Context) ([]*domain.User, error) {
	return s.list, s.err
}
func (s *stubService) Update(ctx context.Context, id, name, email string) (*domain.User, error) {
	return s.user, s.err
}
func (s *stubService) Delete(ctx context.Context, id string) error { return s.err }
func (s *stubService) Count(ctx context.Context) (int64, error)    { return 0, s.err }

// ---------- ตัวช่วยยิง request ปลอม ----------

func newEcho() *echo.Echo {
	e := echo.New()
	e.Validator = validator.New()
	return e
}

func doRequest(e *echo.Echo, h echo.HandlerFunc, method, path, body string, params map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	c := e.NewContext(req, rec)
	for k, v := range params {
		c.SetParamNames(k)
		c.SetParamValues(v)
	}

	if err := h(c); err != nil {
		// Echo จะแปลง HTTPError เป็น response เองเมื่อรันผ่าน router
		// ตอนเรียก handler ตรง ๆ ต้องเรียก error handler เอง
		e.HTTPErrorHandler(err, c)
	}
	return rec
}

func sampleUser() *domain.User {
	return &domain.User{
		ID:        "abc123",
		Name:      "Kraiwit",
		Email:     "kraiwit@mail.com",
		Password:  "$2a$10$hashed",
		CreatedAt: time.Now().UTC(),
	}
}

// ---------- Register ----------

func TestRegister_Created(t *testing.T) {
	e := newEcho()
	h := rest.NewUserHandler(&stubService{user: sampleUser()})

	rec := doRequest(e, h.Register, http.MethodPost, "/api/v1/auth/register",
		`{"name":"Kraiwit","email":"kraiwit@mail.com","password":"secret1234"}`, nil)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

// เคสสำคัญที่สุดของไฟล์นี้: password ต้องไม่หลุดออก response
func TestRegister_NeverLeaksPassword(t *testing.T) {
	e := newEcho()
	h := rest.NewUserHandler(&stubService{user: sampleUser()})

	rec := doRequest(e, h.Register, http.MethodPost, "/api/v1/auth/register",
		`{"name":"Kraiwit","email":"kraiwit@mail.com","password":"secret1234"}`, nil)

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if _, exists := body["password"]; exists {
		t.Error("response must not contain password field")
	}
	if strings.Contains(rec.Body.String(), "$2a$") {
		t.Error("response leaked password hash")
	}
}

func TestRegister_ValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"missing name", `{"email":"a@mail.com","password":"secret1234"}`},
		{"invalid email", `{"name":"Ab","email":"not-an-email","password":"secret1234"}`},
		{"password too short", `{"name":"Ab","email":"a@mail.com","password":"123"}`},
		{"name too short", `{"name":"A","email":"a@mail.com","password":"secret1234"}`},
		{"malformed json", `{"name":`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := newEcho()
			h := rest.NewUserHandler(&stubService{user: sampleUser()})

			rec := doRequest(e, h.Register, http.MethodPost, "/api/v1/auth/register", tc.body, nil)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestRegister_DuplicateEmail_Returns409(t *testing.T) {
	e := newEcho()
	h := rest.NewUserHandler(&stubService{err: domain.ErrEmailAlreadyExists})

	rec := doRequest(e, h.Register, http.MethodPost, "/api/v1/auth/register",
		`{"name":"Kraiwit","email":"kraiwit@mail.com","password":"secret1234"}`, nil)

	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rec.Code)
	}
}

// ---------- Login ----------

func TestLogin_OK(t *testing.T) {
	e := newEcho()
	h := rest.NewUserHandler(&stubService{auth: &port.AuthResult{
		Token:     "jwt-token",
		ExpiresAt: time.Now().Add(time.Hour),
		User:      sampleUser(),
	}})

	rec := doRequest(e, h.Login, http.MethodPost, "/api/v1/auth/login",
		`{"email":"kraiwit@mail.com","password":"secret1234"}`, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)

	if body["access_token"] != "jwt-token" {
		t.Errorf("expected token in response, got %v", body["access_token"])
	}
	if body["token_type"] != "Bearer" {
		t.Errorf("expected token_type Bearer, got %v", body["token_type"])
	}
}

func TestLogin_BadCredentials_Returns401(t *testing.T) {
	e := newEcho()
	h := rest.NewUserHandler(&stubService{err: domain.ErrInvalidCredentials})

	rec := doRequest(e, h.Login, http.MethodPost, "/api/v1/auth/login",
		`{"email":"kraiwit@mail.com","password":"wrong"}`, nil)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

// ---------- error mapping ----------

func TestErrorMapping(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{"not found", domain.ErrNotFound, http.StatusNotFound},
		{"invalid id", domain.ErrInvalidID, http.StatusBadRequest},
		{"duplicate", domain.ErrEmailAlreadyExists, http.StatusConflict},
		{"unknown error", errUnexpected, http.StatusInternalServerError},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := newEcho()
			h := rest.NewUserHandler(&stubService{err: tc.err})

			rec := doRequest(e, h.GetByID, http.MethodGet, "/api/v1/users/x", "",
				map[string]string{"id": "x"})

			if rec.Code != tc.expected {
				t.Errorf("expected %d, got %d", tc.expected, rec.Code)
			}
		})
	}
}

// internal error ต้องไม่หลุดรายละเอียดออกไป
func TestInternalError_DoesNotLeakDetails(t *testing.T) {
	e := newEcho()
	h := rest.NewUserHandler(&stubService{err: errUnexpected})

	rec := doRequest(e, h.List, http.MethodGet, "/api/v1/users", "", nil)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "connection string") ||
		strings.Contains(rec.Body.String(), errUnexpected.Error()) {
		t.Errorf("500 response leaked internal detail: %s", rec.Body.String())
	}
}

// ---------- CRUD ----------

func TestList_OK(t *testing.T) {
	e := newEcho()
	h := rest.NewUserHandler(&stubService{list: []*domain.User{sampleUser(), sampleUser()}})

	rec := doRequest(e, h.List, http.MethodGet, "/api/v1/users", "", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body struct {
		Data  []map[string]any `json:"data"`
		Total int              `json:"total"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)

	if body.Total != 2 || len(body.Data) != 2 {
		t.Errorf("expected 2 users, got total=%d len=%d", body.Total, len(body.Data))
	}
}

func TestUpdate_RequiresAtLeastOneField(t *testing.T) {
	e := newEcho()
	h := rest.NewUserHandler(&stubService{user: sampleUser()})

	rec := doRequest(e, h.Update, http.MethodPut, "/api/v1/users/abc123", `{}`,
		map[string]string{"id": "abc123"})

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty update, got %d", rec.Code)
	}
}

func TestDelete_NoContent(t *testing.T) {
	e := newEcho()
	h := rest.NewUserHandler(&stubService{})

	rec := doRequest(e, h.Delete, http.MethodDelete, "/api/v1/users/abc123", "",
		map[string]string{"id": "abc123"})

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rec.Code)
	}
}
