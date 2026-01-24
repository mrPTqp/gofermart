package service

import (
	"context"
	"errors"
	"testing"

	"github.com/mrPTqp/gofermart/internal/model"
	"github.com/mrPTqp/gofermart/internal/repository"
)

var _ repository.AccountRepository = (*mockAccountRepository)(nil)

type mockAccountRepository struct {
	balance       *model.Balance
	withdrawals   []model.Withdrawal
	withdrawErr   error
	getErr        error
	increaseErr   error
	addAccrualErr error
}

func (m *mockAccountRepository) GetCurrentBalance(ctx context.Context, userID int64) (*model.Balance, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.balance, nil
}

func (m *mockAccountRepository) AddAccrual(ctx context.Context, orderNumber string, userID int64, accrual float64) error {
	return m.addAccrualErr
}

func (m *mockAccountRepository) Withdraw(ctx context.Context, userID int64, order string, sum float64) error {
	return m.withdrawErr
}

func (m *mockAccountRepository) GetWithdrawals(ctx context.Context, userID int64) ([]model.Withdrawal, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.withdrawals, nil
}

func (m *mockAccountRepository) IncreaseBalance(ctx context.Context, userID int64, orderNumber string, amount float64) error {
	return m.increaseErr
}

func TestAccountService_GetBalance(t *testing.T) {
	tests := []struct {
		name      string
		mock      *mockAccountRepository
		wantErr   bool
		wantEmpty bool
	}{
		{
			name: "balance found",
			mock: &mockAccountRepository{balance: &model.Balance{
				Current: 100.50, Withdrawn: 10.0}},
			wantErr: false,
		},
		{
			name:    "repo error",
			mock:    &mockAccountRepository{getErr: errors.New("db error")},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &AccountServiceDefault{repo: tt.mock}
			balance, err := svc.GetBalance(context.Background(), 1)

			if (err != nil) != tt.wantErr {
				t.Fatalf("GetBalance() error = %v, wantErr = %v", err, tt.wantErr)
			}
			if !tt.wantErr && balance.Current == 0 && tt.wantEmpty {
				t.Error("Expected zero balance but not error")
			}
		})
	}
}

func TestAccountService_Withdraw(t *testing.T) {
	tests := []struct {
		name        string
		userID      int64
		order       string
		sum         float64
		balance     *model.Balance
		withdrawErr error
		getErr      error
		wantErr     bool
		wantMsg     string
	}{
		{
			name:    "successful withdrawal",
			userID:  1,
			order:   "12345",
			sum:     50.0,
			balance: &model.Balance{Current: 100.0},
			wantErr: false,
		},
		{
			name:    "insufficient funds",
			userID:  1,
			order:   "12345",
			sum:     150.0,
			balance: &model.Balance{Current: 100.0},
			wantErr: true,
			wantMsg: "insufficient funds",
		},
		{
			name:    "negative amount",
			userID:  1,
			order:   "12345",
			sum:     -10.0,
			wantErr: true,
		},
		{
			name:    "repo error on get",
			userID:  1,
			order:   "12345",
			sum:     50.0,
			getErr:  errors.New("db error"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockAccountRepository{
				balance:       tt.balance,
				withdrawErr:   tt.withdrawErr,
				getErr:        tt.getErr,
				addAccrualErr: nil,
			}
			svc := NewAccountService(repo)

			err := svc.Withdraw(context.Background(), tt.userID, tt.order, tt.sum)

			if (err != nil) != tt.wantErr {
				t.Fatalf("Withdraw() error = %v, wantErr = %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.wantMsg != "" {
				if err.Error() != tt.wantMsg {
					t.Errorf("Expected error %q, got %q", tt.wantMsg, err.Error())
				}
			}
		})
	}

}
