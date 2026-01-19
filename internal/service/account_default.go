// internal/service/account.go
package service

import (
	"context"
	"errors"

	"github.com/mrPTqp/gofermart/internal/floatutils"
	"github.com/mrPTqp/gofermart/internal/model"
	"github.com/mrPTqp/gofermart/internal/repository"
)

type AccountServiceDefault struct {
	repo repository.AccountRepository
}

func NewAccountService(repo repository.AccountRepository) *AccountServiceDefault {
	return &AccountServiceDefault{repo: repo}
}

func (s *AccountServiceDefault) GetBalance(ctx context.Context, userID int64) (model.Balance, error) {
	balance, err := s.repo.GetCurrentBalance(ctx, userID)
	if err != nil {
		return model.Balance{}, err
	}
	return *balance, nil
}

func (s *AccountServiceDefault) Withdraw(ctx context.Context, userID int64, order string, sum float64) error {
	if sum <= 0 {
		return errors.New("withdrawal amount must be positive")
	}

	roundedSum := floatutils.Round(sum, 2)

	balance, err := s.repo.GetCurrentBalance(ctx, userID)
	if err != nil {
		return err
	}

	if balance.Current < roundedSum {
		return repository.ErrInsufficientFunds
	}

	return s.repo.Withdraw(ctx, userID, order, roundedSum)
}

func (s *AccountServiceDefault) GetWithdrawals(ctx context.Context, userID int64) ([]model.Withdrawal, error) {
	withdrawals, err := s.repo.GetWithdrawals(ctx, userID)
	if err != nil {
		return nil, err
	}
	return withdrawals, nil
}

func (s *AccountServiceDefault) IncreaseBalance(ctx context.Context, userID int64, orderNumber string, amount float64) error {
	if amount <= 0 {
		return errors.New("amount must be positive")
	}
	roundedAmount := floatutils.Round(amount, 2)
	return s.repo.IncreaseBalance(ctx, userID, orderNumber, roundedAmount)
}
