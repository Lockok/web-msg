package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Lockok/web-msg/internal/auth"
	"github.com/Lockok/web-msg/internal/domain"
	"github.com/google/uuid"
)

type mockUserService struct {
	err error
}

func (m *mockUserService) Register(
	ctx context.Context,
	username string,
	password string,
) (*domain.User, error) {
	if m.err != nil {
		return nil, m.err
	}

	return &domain.User{
		ID:       uuid.New(),
		Username: username,
	}, nil
}

func (m *mockUserService) Login(
	ctx context.Context,
	username string,
	password string,
) (*domain.User, error) {
	if m.err != nil {
		return nil, m.err
	}

	return &domain.User{
		ID:       uuid.New(),
		Username: username,
	}, nil
}

// //////////////////////////////// REGISTER TESTS ////////////////////////////////////////////////////////
func TestHandler_Register(t *testing.T) {
	service := &mockUserService{}
	jwtManager := auth.NewJWTManager("test-secret") // Assuming you have a mock or real JWTManager implementation
	handler := NewHandler(service, jwtManager)

	body := `{"username":"alex","password":"12345678"}`

	req := httptest.NewRequest(
		http.MethodPost,
		"/register",
		strings.NewReader(body),
	)

	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}
}

func TestHandler_RegisterUsernameExists(t *testing.T) {
	service := &mockUserService{
		err: domain.ErrUsernameExists,
	}

	jwtManager := auth.NewJWTManager("test-secret")
	handler := NewHandler(service, jwtManager)

	body := `{"username":"alex","password":"12345678"}`

	req := httptest.NewRequest(
		http.MethodPost,
		"/register",
		strings.NewReader(body),
	)

	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusConflict,
			rec.Code,
		)
	}
}

func TestHandler_RegisterInvalidJSON(t *testing.T) {
	service := &mockUserService{}
	jwtManager := auth.NewJWTManager("test-secret")
	handler := NewHandler(service, jwtManager)

	body := `{"username":"alex","password":`

	req := httptest.NewRequest(
		http.MethodPost,
		"/register",
		strings.NewReader(body),
	)

	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusBadRequest,
			rec.Code,
		)
	}
}

func TestHandler_RegisterEmptyFields(t *testing.T) {
	service := &mockUserService{}
	jwtManager := auth.NewJWTManager("test-secret")
	handler := NewHandler(service, jwtManager)

	body := `{"username":"","password":""}`

	req := httptest.NewRequest(
		http.MethodPost,
		"/register",
		strings.NewReader(body),
	)

	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusBadRequest,
			rec.Code,
		)
	}
}

//  ////////////////////////// LOGIN TEST ///////////////////////////////////////////////////////////

func TestHandler_Login(t *testing.T) {
	service := &mockUserService{}
	jwtManager := auth.NewJWTManager("test-secret")
	handler := NewHandler(service, jwtManager)

	body := `{"username":"alex","password":"12345678"}`

	req := httptest.NewRequest(
		http.MethodPost,
		"/login",
		strings.NewReader(body),
	)

	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusOK,
			rec.Code,
		)
	}
}

func TestHandler_LoginInvalidCredentials(t *testing.T) {
	service := &mockUserService{
		err: domain.ErrInvalidCredentials,
	}

	jwtManager := auth.NewJWTManager("test-secret")
	handler := NewHandler(service, jwtManager)

	body := `{"username":"alex","password":"wrong-password"}`

	req := httptest.NewRequest(
		http.MethodPost,
		"/login",
		strings.NewReader(body),
	)

	rec := httptest.NewRecorder()

	handler.Login(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusUnauthorized,
			rec.Code,
		)
	}
}

func TestHandler_RegisterInvalidInput(t *testing.T) {
	service := &mockUserService{
		err: domain.ErrInvalidInput,
	}

	handler := NewHandler(service, auth.NewJWTManager("test-secret"))

	req := httptest.NewRequest(
		http.MethodPost,
		"/register",
		strings.NewReader(`{"username":"ab","password":"12345678"}`),
	)
	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}
