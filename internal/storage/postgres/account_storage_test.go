package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/mrPTqp/gofermart/internal/repository"
	"go.uber.org/zap/zaptest"
)

func newAccountStorageWithMockDB(t *testing.T) (*AccountStorage, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}

	logger := zaptest.NewLogger(t)
	storage, err := NewAccountStorage(db, logger)
	if err != nil {
		t.Fatalf("failed to create AccountStorage: %v", err)
	}

	return storage, mock
}

func TestAccountStorage_GetCurrentBalance(t *testing.T) {
	storage, mock := newAccountStorageWithMockDB(t)
	defer mock.ExpectClose()

	tests := []struct {
		name            string
		userID          int64
		mockSetup       func()
		expectedCurrent float64
		expectedWithdrawn float64
		expectedError   error
	}{
		{
			name:   "balance with positive funds",
			userID: 1,
			mockSetup: func() {
				rows := sqlmock.NewRows([]string{"current", "withdrawn"}).
					AddRow(500.00, 200.00)
				mock.ExpectQuery(`^SELECT SUM\(difference\)::FLOAT8 / 100\.0, \(SUM\(CASE WHEN difference < 0 THEN -difference ELSE 0 END\)::FLOAT8 / 100\.0\) FROM public\.t_account WHERE user_id = \$1$`).
					WithArgs(int64(1)).
					WillReturnRows(rows)
			},
			expectedCurrent:   500.00,
			expectedWithdrawn: 200.00,
			expectedError:     nil,
		},
		{
			name:   "zero balance",
			userID: 2,
			mockSetup: func() {
				rows := sqlmock.NewRows([]string{"current", "withdrawn"}).
					AddRow(nil, nil)
				mock.ExpectQuery(`^SELECT SUM\(difference\)::FLOAT8 / 100\.0, \(SUM\(CASE WHEN difference < 0 THEN -difference ELSE 0 END\)::FLOAT8 / 100\.0\) FROM public\.t_account WHERE user_id = \$1$`).
					WithArgs(int64(2)).
					WillReturnRows(rows)
			},
			expectedCurrent:   0.0,
			expectedWithdrawn: 0.0,
			expectedError:     nil,
		},
		{
			name:   "db error",
			userID: 3,
			mockSetup: func() {
				mock.ExpectQuery(`^SELECT SUM\(difference\)::FLOAT8 / 100\.0, \(SUM\(CASE WHEN difference < 0 THEN -difference ELSE 0 END\)::FLOAT8 / 100\.0\) FROM public\.t_account WHERE user_id = \$1$`).
					WithArgs(int64(3)).
					WillReturnError(errors.New("db unreachable"))
			},
			expectedCurrent:   0.0,
			expectedWithdrawn: 0.0,
			expectedError:     errors.New("db unreachable"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			balance, err := storage.GetCurrentBalance(context.Background(), tt.userID)

			if tt.expectedError != nil {
				if err == nil || err.Error() != tt.expectedError.Error() {
					t.Errorf("expected error %v, got %v", tt.expectedError, err)
				}
				if balance != nil {
					t.Errorf("expected nil balance, got %+v", balance)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
				if balance == nil {
					t.Error("expected balance, got nil")
				} else {
					if balance.Current != tt.expectedCurrent {
						t.Errorf("expected Current = %v, got %v", tt.expectedCurrent, balance.Current)
					}
					if balance.Withdrawn != tt.expectedWithdrawn {
						t.Errorf("expected Withdrawn = %v, got %v", tt.expectedWithdrawn, balance.Withdrawn)
					}
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestAccountStorage_AddAccrual(t *testing.T) {
	storage, mock := newAccountStorageWithMockDB(t)
	defer mock.ExpectClose()

	tests := []struct {
		name        string
		orderNumber string
		userID      int64
		accrual     float64
		mockSetup   func()
		expectedErr error
	}{
		{
			name:        "accrual added",
			orderNumber: "12345",
			userID:      1,
			accrual:     500.55,
			mockSetup: func() {
				mock.ExpectExec(`^INSERT INTO public\.t_account \(user_id, order_number, difference\) VALUES \(\$1, \$2, \$3\)$`).
					WithArgs(int64(1), "12345", 50055).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			expectedErr: nil,
		},
		{
			name:        "db error",
			orderNumber: "67890",
			userID:      2,
			accrual:     100.00,
			mockSetup: func() {
				mock.ExpectExec(`^INSERT INTO public\.t_account \(user_id, order_number, difference\) VALUES \(\$1, \$2, \$3\)$`).
					WithArgs(int64(2), "67890", 10000).
					WillReturnError(errors.New("insert failed"))
			},
			expectedErr: errors.New("insert failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			err := storage.AddAccrual(context.Background(), tt.orderNumber, tt.userID, tt.accrual)

			if tt.expectedErr == nil {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.expectedErr)
				} else if err.Error() != tt.expectedErr.Error() {
					t.Errorf("expected %v, got %v", tt.expectedErr, err)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestAccountStorage_Withdraw(t *testing.T) {
	storage, mock := newAccountStorageWithMockDB(t)
	defer mock.ExpectClose()

	tests := []struct {
		name        string
		userID      int64
		order       string
		sum         float64
		mockSetup   func()
		expectedErr error
	}{
		{
			name:  "withdrawal successful",
			userID: 1,
			order: "ORDER-001",
			sum:   100.00,
			mockSetup: func() {
				mock.ExpectQuery(`^SELECT COALESCE\(SUM\(difference\), 0\) FROM public\.t_account WHERE user_id = \$1$`).
					WithArgs(int64(1)).
					WillReturnRows(sqlmock.NewRows([]string{"sum"}).AddRow(15000))

				mock.ExpectExec(`^INSERT INTO public\.t_account \(user_id, order_number, difference\) VALUES \(\$1, \$2, \$3\)$`).
					WithArgs(int64(1), "ORDER-001", -10000).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			expectedErr: nil,
		},
		{
			name:  "insufficient funds",
			userID: 2,
			order: "ORDER-002",
			sum:   300.00,
			mockSetup: func() {
				mock.ExpectQuery(`^SELECT COALESCE\(SUM\(difference\), 0\) FROM public\.t_account WHERE user_id = \$1$`).
					WithArgs(int64(2)).
					WillReturnRows(sqlmock.NewRows([]string{"sum"}).AddRow(20000))
			},
			expectedErr: repository.ErrInsufficientFunds,
		},
		{
			name:  "db error on balance check",
			userID: 3,
			order: "ORDER-003",
			sum:   50.00,
			mockSetup: func() {
				mock.ExpectQuery(`^SELECT COALESCE\(SUM\(difference\), 0\) FROM public\.t_account WHERE user_id = \$1$`).
					WithArgs(int64(3)).
					WillReturnError(errors.New("query failed"))
			},
			expectedErr: errors.New("query failed"),
		},
		{
			name:  "db error on insert",
			userID: 4,
			order: "ORDER-004",
			sum:   100.00,
			mockSetup: func() {
				mock.ExpectQuery(`^SELECT COALESCE\(SUM\(difference\), 0\) FROM public\.t_account WHERE user_id = \$1$`).
					WithArgs(int64(4)).
					WillReturnRows(sqlmock.NewRows([]string{"sum"}).AddRow(20000))

				mock.ExpectExec(`^INSERT INTO public\.t_account \(user_id, order_number, difference\) VALUES \(\$1, \$2, \$3\)$`).
					WithArgs(int64(4), "ORDER-004", -10000).
					WillReturnError(errors.New("insert failed"))
			},
			expectedErr: errors.New("insert failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			err := storage.Withdraw(context.Background(), tt.userID, tt.order, tt.sum)

			if tt.expectedErr == nil {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.expectedErr)
				} else if !errors.Is(err, tt.expectedErr) && err.Error() != tt.expectedErr.Error() {
					t.Errorf("expected %v, got %v", tt.expectedErr, err)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestAccountStorage_GetWithdrawals(t *testing.T) {
	storage, mock := newAccountStorageWithMockDB(t)
	defer mock.ExpectClose()

	now := time.Now()

	tests := []struct {
		name          string
		userID        int64
		mockSetup     func()
		expectedLen   int
		expectedError error
	}{
		{
			name:   "withdrawals exist",
			userID: 1,
			mockSetup: func() {
				rows := sqlmock.NewRows([]string{"order_number", "sum", "created_at"}).
					AddRow("ORDER-001", 100.00, now).
					AddRow("ORDER-002", 50.50, now)

				mock.ExpectQuery(`^SELECT order_number, \(-difference::FLOAT8 / 100\.0\), created_at FROM public\.t_account WHERE user_id = \$1 AND difference < 0 ORDER BY created_at DESC$`).
					WithArgs(int64(1)).
					WillReturnRows(rows)
			},
			expectedLen:   2,
			expectedError: nil,
		},
		{
			name:   "no withdrawals",
			userID: 2,
			mockSetup: func() {
				rows := sqlmock.NewRows([]string{"order_number", "sum", "created_at"})
				mock.ExpectQuery(`^SELECT order_number, \(-difference::FLOAT8 / 100\.0\), created_at FROM public\.t_account WHERE user_id = \$1 AND difference < 0 ORDER BY created_at DESC$`).
					WithArgs(int64(2)).
					WillReturnRows(rows)
			},
			expectedLen:   0,
			expectedError: nil,
		},
		{
			name:   "db error",
			userID: 3,
			mockSetup: func() {
				mock.ExpectQuery(`^SELECT order_number, \(-difference::FLOAT8 / 100\.0\), created_at FROM public\.t_account WHERE user_id = \$1 AND difference < 0 ORDER BY created_at DESC$`).
					WithArgs(int64(3)).
					WillReturnError(errors.New("query failed"))
			},
			expectedLen:   0,
			expectedError: errors.New("query failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			withdrawals, err := storage.GetWithdrawals(context.Background(), tt.userID)

			if tt.expectedError != nil {
				if err == nil || err.Error() != tt.expectedError.Error() {
					t.Errorf("expected error %v, got %v", tt.expectedError, err)
				}
				if withdrawals != nil {
					t.Errorf("expected nil withdrawals, got %d entries", len(withdrawals))
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
				if len(withdrawals) != tt.expectedLen {
					t.Errorf("expected %d withdrawals, got %d", tt.expectedLen, len(withdrawals))
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestAccountStorage_IncreaseBalance(t *testing.T) {
	storage, mock := newAccountStorageWithMockDB(t)
	defer mock.ExpectClose()

	tests := []struct {
		name        string
		userID      int64
		orderNumber string
		amount      float64
		mockSetup   func()
		expectedErr error
	}{
		{
			name:        "balance increased",
			userID:      1,
			orderNumber: "ORDER-100",
			amount:      250.75,
			mockSetup: func() {
				mock.ExpectExec(`^INSERT INTO public\.t_account \(user_id, order_number, difference\) VALUES \(\$1, \$2, \$3\)$`).
					WithArgs(int64(1), "ORDER-100", 25075).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			expectedErr: nil,
		},
		{
			name:        "db error",
			userID:      2,
			orderNumber: "ORDER-200",
			amount:      100.00,
			mockSetup: func() {
				mock.ExpectExec(`^INSERT INTO public\.t_account \(user_id, order_number, difference\) VALUES \(\$1, \$2, \$3\)$`).
					WithArgs(int64(2), "ORDER-200", 10000).
					WillReturnError(errors.New("insert failed"))
			},
			expectedErr: errors.New("insert failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			err := storage.IncreaseBalance(context.Background(), tt.userID, tt.orderNumber, tt.amount)

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
