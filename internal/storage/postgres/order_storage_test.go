package postgres

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/mrPTqp/gofermart/internal/repository"
	"github.com/mrPTqp/gofermart/internal/model"
	"go.uber.org/zap/zaptest"
)

func newOrderStorageWithMockDB(t *testing.T) (*OrderStorage, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}

	logger := zaptest.NewLogger(t)
	storage, err := NewOrderStorage(db, logger)
	if err != nil {
		t.Fatalf("failed to create OrderStorage: %v", err)
	}

	return storage, mock
}

func TestOrderStorage_Create(t *testing.T) {
	storage, mock := newOrderStorageWithMockDB(t)
	defer mock.ExpectClose()

	tests := []struct {
		name         string
		number       string
		userID       int64
		mockSetup    func()
		expectedErr  error
	}{
		{
			name:   "successful creation",
			number: "1234567890",
			userID: 1,
			mockSetup: func() {
				// SELECT: заказа нет
				mock.ExpectQuery(`^SELECT user_id FROM public\.t_order WHERE number = \$1$`).
					WithArgs("1234567890").
					WillReturnError(sql.ErrNoRows)

				// INSERT: вставляем заказ
				mock.ExpectExec(`^INSERT INTO public\.t_order \(number, user_id\) VALUES \(\$1, \$2\)$`).
					WithArgs("1234567890", int64(1)).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			expectedErr: nil,
		},
		{
			name:   "order already exists for user",
			number: "1234567890",
			userID: 1,
			mockSetup: func() {
				// SELECT: заказ найден, user_id = 1
				rows := sqlmock.NewRows([]string{"user_id"}).AddRow(1)
				mock.ExpectQuery(`^SELECT user_id FROM public\.t_order WHERE number = \$1$`).
					WithArgs("1234567890").
					WillReturnRows(rows)
			},
			expectedErr: repository.ErrOrderExists,
		},
		{
			name:   "order taken by another user",
			number: "1234567890",
			userID: 1,
			mockSetup: func() {
				// SELECT: заказ найден, user_id = 2
				rows := sqlmock.NewRows([]string{"user_id"}).AddRow(2)
				mock.ExpectQuery(`^SELECT user_id FROM public\.t_order WHERE number = \$1$`).
					WithArgs("1234567890").
					WillReturnRows(rows)
			},
			expectedErr: repository.ErrOrderTaken,
		},
		{
			name:   "db error on select",
			number: "1234567890",
			userID: 1,
			mockSetup: func() {
				// SELECT: ошибка БД
				mock.ExpectQuery(`^SELECT user_id FROM public\.t_order WHERE number = \$1$`).
					WithArgs("1234567890").
					WillReturnError(errors.New("db unreachable"))
			},
			expectedErr: errors.New("db unreachable"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			err := storage.Create(context.Background(), tt.number, tt.userID)

			if tt.expectedErr == nil {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.expectedErr)
				} else if err.Error() != tt.expectedErr.Error() && !errors.Is(err, tt.expectedErr) {
					t.Errorf("expected %v, got %v", tt.expectedErr, err)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestOrderStorage_GetByUser(t *testing.T) {
	storage, mock := newOrderStorageWithMockDB(t)
	defer mock.ExpectClose()

	now := time.Now()

	tests := []struct {
		name         string
		userID       int64
		mockSetup    func()
		expectedLen  int
		expectedErr  error
	}{
		{
			name:   "orders found with accrual",
			userID: 1,
			mockSetup: func() {
				rows := sqlmock.NewRows([]string{
					"number", "created_at", "updated_at", "status_code", "last_checked_at", "accrual",
				}).
					AddRow("12345", now, now, "PROCESSED", now, 500.55).
					AddRow("12346", now, now, "NEW", nil, nil)

				mock.ExpectQuery(`^SELECT .* FROM public\.t_order o .* WHERE o\.user_id = \$1 .* ORDER BY o\.created_at DESC$`).
					WithArgs(int64(1)).
					WillReturnRows(rows)
			},
			expectedLen: 2,
			expectedErr: nil,
		},
		{
			name:   "no orders",
			userID: 2,
			mockSetup: func() {
				rows := sqlmock.NewRows([]string{
					"number", "created_at", "updated_at", "status_code", "last_checked_at", "accrual",
				})
				mock.ExpectQuery(`^SELECT .* FROM public\.t_order o .* WHERE o\.user_id = \$1 .* ORDER BY o\.created_at DESC$`).
					WithArgs(int64(2)).
					WillReturnRows(rows)
			},
			expectedLen: 0,
			expectedErr: nil,
		},
		{
			name:   "db error on query",
			userID: 3,
			mockSetup: func() {
				mock.ExpectQuery(`^SELECT .* FROM public\.t_order o .* WHERE o\.user_id = \$1 .* ORDER BY o\.created_at DESC$`).
					WithArgs(int64(3)).
					WillReturnError(errors.New("query failed"))
			},
			expectedLen: 0,
			expectedErr: errors.New("query failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			orders, err := storage.GetByUser(context.Background(), tt.userID)

			if tt.expectedErr != nil {
				if err == nil || err.Error() != tt.expectedErr.Error() {
					t.Errorf("expected error %v, got %v", tt.expectedErr, err)
				}
				if orders != nil {
					t.Errorf("expected nil orders, got %d", len(orders))
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
				if len(orders) != tt.expectedLen {
					t.Errorf("expected %d orders, got %d", tt.expectedLen, len(orders))
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestOrderStorage_GetByNumber(t *testing.T) {
	storage, mock := newOrderStorageWithMockDB(t)
	defer mock.ExpectClose()

	now := time.Now()

	tests := []struct {
		name        string
		number      string
		mockSetup   func()
		expectFound bool
		expectedErr error
	}{
		{
			name:   "order found",
			number: "12345",
			mockSetup: func() {
				rows := sqlmock.NewRows([]string{
					"number", "user_id", "created_at", "updated_at", "status_code", "last_checked_at",
				}).AddRow("12345", int64(1), now, now, "NEW", now)

				mock.ExpectQuery(`^SELECT number, user_id, created_at, updated_at, status_code, last_checked_at FROM public\.t_order WHERE number = \$1$`).
					WithArgs("12345").
					WillReturnRows(rows)
			},
			expectFound: true,
			expectedErr: nil,
		},
		{
			name:   "order not found",
			number: "99999",
			mockSetup: func() {
				mock.ExpectQuery(`^SELECT number, user_id, created_at, updated_at, status_code, last_checked_at FROM public\.t_order WHERE number = \$1$`).
					WithArgs("99999").
					WillReturnError(sql.ErrNoRows)
			},
			expectFound: false,
			expectedErr: repository.ErrNotFound,
		},
		{
			name:   "db error",
			number: "88888",
			mockSetup: func() {
				mock.ExpectQuery(`^SELECT number, user_id, created_at, updated_at, status_code, last_checked_at FROM public\.t_order WHERE number = \$1$`).
					WithArgs("88888").
					WillReturnError(errors.New("db down"))
			},
			expectFound: false,
			expectedErr: errors.New("db down"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			order, err := storage.GetByNumber(context.Background(), tt.number)

			if tt.expectFound {
				if err != nil {
					t.Errorf("expected order, got error: %v", err)
				}
				if order == nil {
					t.Error("expected order, got nil")
					return
				}
				if order.Number != tt.number {
					t.Errorf("expected number %s, got %s", tt.number, order.Number)
				}
			} else {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if tt.expectedErr != nil && err.Error() != tt.expectedErr.Error() {
					t.Errorf("expected %v, got %v", tt.expectedErr, err)
				}
				if order != nil {
					t.Errorf("expected nil order, got %+v", order)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestOrderStorage_UpdateStatus(t *testing.T) {
	storage, mock := newOrderStorageWithMockDB(t)
	defer mock.ExpectClose()

	tests := []struct {
		name        string
		number      string
		status      string
		mockSetup   func()
		expectedErr error
	}{
		{
			name:   "status updated",
			number: "12345",
			status: "PROCESSED",
			mockSetup: func() {
				mock.ExpectExec(`^UPDATE public\.t_order SET status_code = \$1, last_checked_at = NOW\(\), updated_at = NOW\(\) WHERE number = \$2$`).
					WithArgs("PROCESSED", "12345").
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			expectedErr: nil,
		},
		{
			name:   "order not found",
			number: "99999",
			status: "INVALID",
			mockSetup: func() {
				// UPDATE может не затронуть строки — это не ошибка
				mock.ExpectExec(`^UPDATE public\.t_order SET status_code = \$1, last_checked_at = NOW\(\), updated_at = NOW\(\) WHERE number = \$2$`).
					WithArgs("INVALID", "99999").
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			expectedErr: nil,
		},
		{
			name:   "db error",
			number: "88888",
			status: "NEW",
			mockSetup: func() {
				mock.ExpectExec(`^UPDATE public\.t_order SET status_code = \$1, last_checked_at = NOW\(\), updated_at = NOW\(\) WHERE number = \$2$`).
					WithArgs("NEW", "88888").
					WillReturnError(errors.New("connection lost"))
			},
			expectedErr: errors.New("connection lost"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			err := storage.UpdateStatus(context.Background(), tt.number, tt.status)

			if tt.expectedErr == nil {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			} else {
				if err == nil || err.Error() != tt.expectedErr.Error() {
					t.Errorf("expected %v, got %v", tt.expectedErr, err)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestOrderStorage_Update(t *testing.T) {
	storage, mock := newOrderStorageWithMockDB(t)
	defer mock.ExpectClose()

	order := &model.Order{
		Number:     "12345",
		StatusCode: "PROCESSED",
	}

	tests := []struct {
		name        string
		order       *model.Order
		mockSetup   func()
		expectedErr error
	}{
		{
			name:  "update success",
			order: order,
			mockSetup: func() {
				mock.ExpectExec(`^UPDATE public\.t_order SET status_code = \$1, last_checked_at = NOW\(\), updated_at = NOW\(\) WHERE number = \$2`).
					WithArgs("PROCESSED", "12345").
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			expectedErr: nil,
		},
		{
			name:  "db error",
			order: order,
			mockSetup: func() {
				mock.ExpectExec(`^UPDATE public\.t_order SET status_code = \$1, last_checked_at = NOW\(\), updated_at = NOW\(\) WHERE number = \$2`).
					WithArgs("PROCESSED", "12345").
					WillReturnError(errors.New("db error"))
			},
			expectedErr: errors.New("db error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			err := storage.Update(context.Background(), tt.order)

			if tt.expectedErr == nil {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			} else {
				if err == nil || err.Error() != tt.expectedErr.Error() {
					t.Errorf("expected %v, got %v", tt.expectedErr, err)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestOrderStorage_StreamOrdersForProcessing(t *testing.T) {
	storage, mock := newOrderStorageWithMockDB(t)
	defer mock.ExpectClose()

	now := time.Now()

	tests := []struct {
		name        string
		mockSetup   func()
		processor   repository.OrderProcessor
		expectedErr error
	}{
		{
			name: "orders processed successfully",
			mockSetup: func() {
				rows := sqlmock.NewRows([]string{
					"number", "user_id", "created_at", "updated_at", "status_code", "last_checked_at",
				}).AddRow("12345", int64(1), now, now, "NEW", now)
				mock.ExpectQuery(`^SELECT number, user_id, created_at, updated_at, status_code, last_checked_at FROM public\.t_order WHERE status_code IN \('NEW', 'PROCESSING'\) ORDER BY created_at ASC`).
					WillReturnRows(rows)
			},
			processor: func(order model.Order) error {
				if order.Number != "12345" {
					t.Errorf("expected number 12345, got %s", order.Number)
				}
				return nil
			},
			expectedErr: nil,
		},
		{
			name: "processor returns error",
			mockSetup: func() {
				rows := sqlmock.NewRows([]string{
					"number", "user_id", "created_at", "updated_at", "status_code", "last_checked_at",
				}).AddRow("12345", int64(1), now, now, "NEW", now)
				mock.ExpectQuery(`^SELECT number, user_id, created_at, updated_at, status_code, last_checked_at FROM public\.t_order WHERE status_code IN \('NEW', 'PROCESSING'\) ORDER BY created_at ASC`).
					WillReturnRows(rows)
			},
			processor: func(order model.Order) error {
				return errors.New("processing failed")
			},
			expectedErr: errors.New("processing failed"),
		},
		{
			name: "db query error",
			mockSetup: func() {
				mock.ExpectQuery(`^SELECT number, user_id, created_at, updated_at, status_code, last_checked_at FROM public\.t_order WHERE status_code IN \('NEW', 'PROCESSING'\) ORDER BY created_at ASC`).
					WillReturnError(errors.New("db error"))
			},
			processor:   func(order model.Order) error { return nil },
			expectedErr: errors.New("db error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			err := storage.StreamOrdersForProcessing(context.Background(), tt.processor)

			if tt.expectedErr == nil {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			} else {
				if err == nil || err.Error() != tt.expectedErr.Error() {
					t.Errorf("expected %v, got %v", tt.expectedErr, err)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}
