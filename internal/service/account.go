package service

import (
	"context"

	"github.com/mrPTqp/gofermart/internal/model"
)

type AccountService interface {
	GetBalance(ctx context.Context, userID int64) (model.Balance, error)
	Withdraw(ctx context.Context, userID int64, order string, sum float64) error
	GetWithdrawals(ctx context.Context, userID int64) ([]model.Withdrawal, error)
	IncreaseBalance(ctx context.Context, userID int64, orderNumber string, amount float64) error
}