// internal/handler/handler_test.go
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mrPTqp/gofermart/internal/config"
	"github.com/mrPTqp/gofermart/internal/model"
	"github.com/mrPTqp/gofermart/internal/repository"
	"go.uber.org/zap/zaptest"
)

const testJWTSecret = "test_secret"

type MockUserService struct {
	RegisterFunc func(ctx context.Context, login, password string) (int64, string, error)
	LoginFunc    func(ctx context.Context, login, password string) (int64, string, error)
}

func (m *MockUserService) Register(ctx context.Context, login, password string) (int64, string, error) {
	return m.RegisterFunc(ctx, login, password)
}

func (m *MockUserService) Login(ctx context.Context, login, password string) (int64, string, error) {
	return m.LoginFunc(ctx, login, password)
}

type MockOrderService struct {
	UploadOrderFunc   func(ctx context.Context, userID int64, orderNum string) error
	GetUserOrdersFunc func(ctx context.Context, userID int64) ([]model.Order, error)
}

func (m *MockOrderService) UploadOrder(ctx context.Context, userID int64, orderNum string) error {
	return m.UploadOrderFunc(ctx, userID, orderNum)
}

func (m *MockOrderService) GetUserOrders(ctx context.Context, userID int64) ([]model.Order, error) {
	return m.GetUserOrdersFunc(ctx, userID)
}

type MockAccountService struct {
	GetBalanceFunc       func(ctx context.Context, userID int64) (model.Balance, error)
	WithdrawFunc         func(ctx context.Context, userID int64, order string, sum float64) error
	GetWithdrawalsFunc   func(ctx context.Context, userID int64) ([]model.Withdrawal, error)
	IncreaseBalanceFunc  func(ctx context.Context, userID int64, orderNumber string, amount float64) error
}

func (m *MockAccountService) GetBalance(ctx context.Context, userID int64) (model.Balance, error) {
	return m.GetBalanceFunc(ctx, userID)
}

func (m *MockAccountService) Withdraw(ctx context.Context, userID int64, order string, sum float64) error {
	return m.WithdrawFunc(ctx, userID, order, sum)
}

func (m *MockAccountService) GetWithdrawals(ctx context.Context, userID int64) ([]model.Withdrawal, error) {
	return m.GetWithdrawalsFunc(ctx, userID)
}

func (m *MockAccountService) IncreaseBalance(ctx context.Context, userID int64, orderNumber string, amount float64) error {
	return m.IncreaseBalanceFunc(ctx, userID, orderNumber, amount)
}

func createTestHandler(
	userService *MockUserService,
	orderService *MockOrderService,
	accountService *MockAccountService,
	t *testing.T,
) *GofermartHandler {
	cfg := &config.Config{
		JWTSecret: testJWTSecret,
	}
	logger := zaptest.NewLogger(t)
	return NewGofermartHandler(userService, orderService, accountService, cfg, logger)
}

func withUserID(req *http.Request, userID int64) *http.Request {
	ctx := context.WithValue(req.Context(), UserIDKeyContext, userID)
	return req.WithContext(ctx)
}

func TestRegister(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		mockRegister   func(ctx context.Context, login, password string) (int64, string, error)
		expectedStatus int
		expectedHeader string
	}{
		{
			name:  "valid registration",
			input: `{"login":"user1","password":"pass123"}`,
			mockRegister: func(ctx context.Context, login, password string) (int64, string, error) {
				return 1, "testtoken", nil
			},
			expectedStatus: http.StatusOK,
			expectedHeader: "Bearer testtoken",
		},
		{
			name:           "invalid json",
			input:          `{"login":}`,
			mockRegister:   func(ctx context.Context, login, password string) (int64, string, error) { return 0, "", nil },
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:  "missing login",
			input: `{"login":"","password":"pass123"}`,
			mockRegister: func(ctx context.Context, login, password string) (int64, string, error) {
				return 0, "", errors.New("login and password required")
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:  "login already exists",
			input: `{"login":"user1","password":"pass123"}`,
			mockRegister: func(ctx context.Context, login, password string) (int64, string, error) {
				return 0, "", repository.ErrLoginExists // ✅ Исправлено
			},
			expectedStatus: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUserSvc := &MockUserService{RegisterFunc: tt.mockRegister}
			handler := createTestHandler(mockUserSvc, nil, nil, t)

			req := httptest.NewRequest(http.MethodPost, "/api/user/register", strings.NewReader(tt.input))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.Register(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
			if tt.expectedHeader != "" {
				if w.Header().Get("Authorization") != tt.expectedHeader {
					t.Errorf("expected Authorization header %q, got %q", tt.expectedHeader, w.Header().Get("Authorization"))
				}
			}
		})
	}
}

func TestLogin(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		mockLogin      func(ctx context.Context, login, password string) (int64, string, error)
		expectedStatus int
		expectedHeader string
	}{
		{
			name:  "valid login",
			input: `{"login":"user1","password":"pass123"}`,
			mockLogin: func(ctx context.Context, login, password string) (int64, string, error) {
				return 1, "testtoken", nil
			},
			expectedStatus: http.StatusOK,
			expectedHeader: "Bearer testtoken",
		},
		{
			name:           "invalid json",
			input:          `{"login":}`,
			mockLogin:      func(ctx context.Context, login, password string) (int64, string, error) { return 0, "", nil },
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:  "invalid credentials",
			input: `{"login":"user1","password":"wrong"}`,
			mockLogin: func(ctx context.Context, login, password string) (int64, string, error) {
				return 0, "", errors.New("invalid credentials")
			},
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUserSvc := &MockUserService{LoginFunc: tt.mockLogin}
			handler := createTestHandler(mockUserSvc, nil, nil, t)

			req := httptest.NewRequest(http.MethodPost, "/api/user/login", strings.NewReader(tt.input))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.Login(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
			if tt.expectedHeader != "" {
				if w.Header().Get("Authorization") != tt.expectedHeader {
					t.Errorf("expected Authorization header %q, got %q", tt.expectedHeader, w.Header().Get("Authorization"))
				}
			}
		})
	}
}

func TestUploadOrder(t *testing.T) {
	tests := []struct {
		name           string
		token          string
		body           string
		mockUpload     func(ctx context.Context, userID int64, orderNum string) error
		expectedStatus int
	}{
		{
			name:   "successful upload",
			token:  "valid",
			body:   "4000001234567899", // ✅ Валидный по Luhn
			mockUpload: func(ctx context.Context, userID int64, orderNum string) error {
				return nil
			},
			expectedStatus: http.StatusAccepted,
		},
		{
			name:           "unauthorized",
			token:          "",
			body:           "4000001234567899",
			mockUpload:     func(ctx context.Context, userID int64, orderNum string) error { return nil },
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:   "already uploaded",
			token:  "valid",
			body:   "4000001234567899",
			mockUpload: func(ctx context.Context, userID int64, orderNum string) error {
				return errors.New("already uploaded")
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "belongs to another user",
			token:  "valid",
			body:   "4000001234567899",
			mockUpload: func(ctx context.Context, userID int64, orderNum string) error {
				return errors.New("another user")
			},
			expectedStatus: http.StatusConflict,
		},
		{
			name:           "empty body",
			token:          "valid",
			body:           "",
			mockUpload:     func(ctx context.Context, userID int64, orderNum string) error { return nil },
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockOrderSvc := &MockOrderService{UploadOrderFunc: tt.mockUpload}
			handler := createTestHandler(nil, mockOrderSvc, nil, t)

			req := httptest.NewRequest(http.MethodPost, "/api/user/orders", strings.NewReader(tt.body))
			if tt.token != "" {
				req = withUserID(req, 1) // ✅ userID в контексте
			}
			w := httptest.NewRecorder()

			handler.UploadOrder(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestGetOrders(t *testing.T) {
	tests := []struct {
		name           string
		token          string
		mockGetOrders  func(ctx context.Context, userID int64) ([]model.Order, error)
		expectedStatus int
		expectBody     bool
	}{
		{
			name:  "orders found",
			token: "valid",
			mockGetOrders: func(ctx context.Context, userID int64) ([]model.Order, error) {
				accrual := 500.0
				return []model.Order{
					{Number: "4000001234567899", StatusCode: "PROCESSED", Accrual: &accrual},
				}, nil
			},
			expectedStatus: http.StatusOK,
			expectBody:     true,
		},
		{
			name:           "no orders",
			token:          "valid",
			mockGetOrders:  func(ctx context.Context, userID int64) ([]model.Order, error) { return nil, nil },
			expectedStatus: http.StatusNoContent,
			expectBody:     false,
		},
		{
			name:           "unauthorized",
			token:          "",
			mockGetOrders:  func(ctx context.Context, userID int64) ([]model.Order, error) { return nil, nil },
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:  "repo error",
			token: "valid",
			mockGetOrders: func(ctx context.Context, userID int64) ([]model.Order, error) {
				return nil, errors.New("db error")
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockOrderSvc := &MockOrderService{GetUserOrdersFunc: tt.mockGetOrders}
			handler := createTestHandler(nil, mockOrderSvc, nil, t)

			req := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
			if tt.token != "" {
				req = withUserID(req, 1) // ✅
			}
			w := httptest.NewRecorder()

			handler.GetOrders(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
			if tt.expectBody && w.Code == http.StatusOK {
				if !strings.Contains(w.Header().Get("Content-Type"), "application/json") {
					t.Error("expected Content-Type application/json")
				}
				if w.Body.Len() == 0 {
					t.Error("expected non-empty body")
				}
			}
		})
	}
}

func TestGetBalance(t *testing.T) {
	tests := []struct {
		name           string
		token          string
		mockGetBalance func(ctx context.Context, userID int64) (model.Balance, error)
		expectedStatus int
	}{
		{
			name:  "balance retrieved",
			token: "valid",
			mockGetBalance: func(ctx context.Context, userID int64) (model.Balance, error) {
				return model.Balance{Current: 1000.50, Withdrawn: 200.00}, nil
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "unauthorized",
			token:          "",
			mockGetBalance: func(ctx context.Context, userID int64) (model.Balance, error) { return model.Balance{}, nil },
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:  "repo error",
			token: "valid",
			mockGetBalance: func(ctx context.Context, userID int64) (model.Balance, error) {
				return model.Balance{}, errors.New("db error")
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAccountSvc := &MockAccountService{GetBalanceFunc: tt.mockGetBalance}
			handler := createTestHandler(nil, nil, mockAccountSvc, t)

			req := httptest.NewRequest(http.MethodGet, "/api/user/balance", nil)
			if tt.token != "" {
				req = withUserID(req, 1) // ✅
			}
			w := httptest.NewRecorder()

			handler.GetBalance(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
			if w.Code == http.StatusOK {
				var balance model.Balance
				if err := json.NewDecoder(w.Body).Decode(&balance); err != nil {
					t.Error("failed to decode balance response")
				}
			}
		})
	}
}

func TestWithdrawBalance(t *testing.T) {
	tests := []struct {
		name           string
		token          string
		input          string
		mockWithdraw   func(ctx context.Context, userID int64, order string, sum float64) error
		expectedStatus int
	}{
		{
			name:   "valid withdrawal",
			token:  "valid",
			input:  `{"order":"4000001234567899","sum":100.50}`,
			mockWithdraw: func(ctx context.Context, userID int64, order string, sum float64) error {
				return nil
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "unauthorized",
			token:          "",
			input:          `{"order":"4000001234567899","sum":100.50}`,
			mockWithdraw:   func(ctx context.Context, userID int64, order string, sum float64) error { return nil },
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:   "invalid order number",
			token:  "valid",
			input:  `{"order":"12345678902","sum":100.50}`,
			mockWithdraw: func(ctx context.Context, userID int64, order string, sum float64) error {
				return nil
			},
			expectedStatus: http.StatusUnprocessableEntity,
		},
		{
			name:   "insufficient funds",
			token:  "valid",
			input:  `{"order":"4000001234567899","sum":100.50}`,
			mockWithdraw: func(ctx context.Context, userID int64, order string, sum float64) error {
				return repository.ErrInsufficientFunds // ✅
			},
			expectedStatus: http.StatusPaymentRequired,
		},
		{
			name:   "invalid sum",
			token:  "valid",
			input:  `{"order":"4000001234567899","sum":0}`,
			mockWithdraw: func(ctx context.Context, userID int64, order string, sum float64) error {
				return errors.New("withdrawal amount must be positive")
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "invalid json",
			token:  "valid",
			input:  `{"order":}`,
			mockWithdraw: func(ctx context.Context, userID int64, order string, sum float64) error {
				return nil
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAccountSvc := &MockAccountService{WithdrawFunc: tt.mockWithdraw}
			handler := createTestHandler(nil, nil, mockAccountSvc, t)

			req := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", strings.NewReader(tt.input))
			if tt.token != "" {
				req = withUserID(req, 1) // ✅
			}
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.WithdrawBalance(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestGetWithdrawals(t *testing.T) {
	tests := []struct {
		name             string
		token            string
		mockGetWithdrawals func(ctx context.Context, userID int64) ([]model.Withdrawal, error)
		expectedStatus   int
		expectBody       bool
	}{
		{
			name:  "withdrawals found",
			token: "valid",
			mockGetWithdrawals: func(ctx context.Context, userID int64) ([]model.Withdrawal, error) {
				return []model.Withdrawal{
					{Order: "4000001234567899", Sum: 500.0, ProcessedAt: time.Now()},
				}, nil
			},
			expectedStatus: http.StatusOK,
			expectBody:     true,
		},
		{
			name:             "no withdrawals",
			token:            "valid",
			mockGetWithdrawals: func(ctx context.Context, userID int64) ([]model.Withdrawal, error) { return nil, nil },
			expectedStatus:   http.StatusNoContent,
		},
		{
			name:             "unauthorized",
			token:            "",
			mockGetWithdrawals: func(ctx context.Context, userID int64) ([]model.Withdrawal, error) { return nil, nil },
			expectedStatus:   http.StatusUnauthorized,
		},
		{
			name:  "repo error",
			token: "valid",
			mockGetWithdrawals: func(ctx context.Context, userID int64) ([]model.Withdrawal, error) {
				return nil, errors.New("db error")
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAccountSvc := &MockAccountService{GetWithdrawalsFunc: tt.mockGetWithdrawals}
			handler := createTestHandler(nil, nil, mockAccountSvc, t)

			req := httptest.NewRequest(http.MethodGet, "/api/user/withdrawals", nil)
			if tt.token != "" {
				req = withUserID(req, 1) // ✅
			}
			w := httptest.NewRecorder()

			handler.GetWithdrawals(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
			if tt.expectBody && w.Code == http.StatusOK {
				if w.Body.Len() == 0 {
					t.Error("expected non-empty body")
				}
				var withdrawals []model.Withdrawal
				if err := json.NewDecoder(&io.LimitedReader{R: w.Body, N: 1 << 20}).Decode(&withdrawals); err != nil {
					t.Errorf("failed to decode withdrawals: %v", err)
				}
			}
		})
	}
}
