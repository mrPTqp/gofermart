package repository

import (
	"context"

	"github.com/mrPTqp/gofermart/internal/model"
)

type AccountRepository interface {
	GetCurrentBalance(ctx context.Context, userID int64) (*model.Balance, error)
	AddAccrual(ctx context.Context, orderNumber string, userID int64, accrual float64) error
	Withdraw(ctx context.Context, userID int64, order string, sum float64) error
	GetWithdrawals(ctx context.Context, userID int64) ([]model.Withdrawal, error)
	IncreaseBalance(ctx context.Context, userID int64, orderNumber string, amount float64) error
}
