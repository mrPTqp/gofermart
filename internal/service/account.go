// internal/service/account.go
package service

import (
	"context"
	"errors"

	"github.com/mrPTqp/gofermart/internal/model"
	"github.com/mrPTqp/gofermart/internal/repository"
)

type AccountService interface {
    GetBalance(ctx context.Context, userID int64) (model.Balance, error)
    Withdraw(ctx context.Context, userID int64, order string, sum float64) error
    GetWithdrawals(ctx context.Context, userID int64) ([]model.Withdrawal, error)
    IncreaseBalance(ctx context.Context, userID int64, amount float64) error
}

type AccountServiceImpl struct {
	repo repository.AccountRepository
}

func NewBalanceServiceImpl(repo repository.AccountRepository) *AccountServiceImpl {
	return &AccountServiceImpl{repo: repo}
}

func (s *AccountServiceImpl) GetBalance(ctx context.Context, userID int64) (model.Balance, error) {
	balance, err := s.repo.GetCurrentBalance(ctx, userID)
	if err != nil {
		return model.Balance{}, err
	}
	return *balance, nil
}

func (s *AccountServiceImpl) Withdraw(ctx context.Context, userID int64, order string, sum float64) error {
	if sum <= 0 {
		return errors.New("withdrawal amount must be positive")
	}

	balance, err := s.repo.GetCurrentBalance(ctx, userID)
	if err != nil {
		return err
	}

	if balance.Current < sum {
		return repository.ErrInsufficientFunds
	}

	return s.repo.Withdraw(ctx, userID, order, sum)
}

func (s *AccountServiceImpl) GetWithdrawals(ctx context.Context, userID int64) ([]model.Withdrawal, error) {
	withdrawals, err := s.repo.GetWithdrawals(ctx, userID)
	if err != nil {
		return nil, err
	}
	return withdrawals, nil
}

func (s *AccountServiceImpl) IncreaseBalance(ctx context.Context, userID int64, amount float64) error {
	if amount <= 0 {
		return errors.New("amount must be positive")
	}
	return s.repo.IncreaseBalance(ctx, userID, amount)
}
