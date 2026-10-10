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
	err           error
	registerCalls int
	loginCalls    int
}

func (m *mockUserService) Register(
	ctx context.Context,
	username string,
	password string,
) (*domain.User, error) {
	m.registerCalls++

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
	m.loginCalls++

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

func TestHandler_AuthRejectsInvalidJSON(t *testing.T) {
	const validBody = `{"username":"alex","password":"12345678"}`

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "empty body",
			body:       "",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "malformed JSON",
			body:       `{"username":`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "unknown field",
			body:       `{"username":"alex","password":"12345678","admin":true}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "second JSON value",
			body:       validBody + ` {}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "trailing garbage",
			body:       validBody + ` garbage`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "oversized JSON",
			body: `{"username":"` +
				strings.Repeat("a", int(maxJSONBodyBytes)) +
				`","password":"12345678"}`,
			wantStatus: http.StatusRequestEntityTooLarge,
		},
		{
			name: "oversized trailing whitespace",
			body: validBody +
				strings.Repeat(" ", int(maxJSONBodyBytes)),
			wantStatus: http.StatusRequestEntityTooLarge,
		},
	}

	for _, path := range []string{"/register", "/login"} {
		t.Run(path, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					service := &mockUserService{}
					handler := NewHandler(
						service,
						auth.NewJWTManager("test-secret"),
					)

					req := httptest.NewRequest(
						http.MethodPost,
						path,
						strings.NewReader(tt.body),
					)
					rec := httptest.NewRecorder()

					if path == "/register" {
						handler.Register(rec, req)
					} else {
						handler.Login(rec, req)
					}

					if rec.Code != tt.wantStatus {
						t.Fatalf(
							"expected status %d, got %d: %s",
							tt.wantStatus,
							rec.Code,
							rec.Body.String(),
						)
					}

					if service.registerCalls != 0 || service.loginCalls != 0 {
						t.Fatal("service must not be called for invalid JSON")
					}
				})
			}
		})
	}
}

func TestHandler_AuthAllowsTrailingWhitespace(t *testing.T) {
	const body = "{\"username\":\"alex\",\"password\":\"12345678\"}\n\t "

	for _, path := range []string{"/register", "/login"} {
		t.Run(path, func(t *testing.T) {
			service := &mockUserService{}
			handler := NewHandler(
				service,
				auth.NewJWTManager("test-secret"),
			)

			req := httptest.NewRequest(
				http.MethodPost,
				path,
				strings.NewReader(body),
			)
			rec := httptest.NewRecorder()

			wantStatus := http.StatusOK

			if path == "/register" {
				handler.Register(rec, req)
				wantStatus = http.StatusCreated
			} else {
				handler.Login(rec, req)
			}

			if rec.Code != wantStatus {
				t.Fatalf(
					"expected status %d, got %d: %s",
					wantStatus,
					rec.Code,
					rec.Body.String(),
				)
			}

			if service.registerCalls+service.loginCalls != 1 {
				t.Fatal("expected exactly one service call")
			}
		})
	}
}
