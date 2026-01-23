package postgres

import (
	"context"
	"database/sql"
	"sync"

	"github.com/mrPTqp/gofermart/internal/floatutils"
	"github.com/mrPTqp/gofermart/internal/model"
	"github.com/mrPTqp/gofermart/internal/repository"
	"go.uber.org/zap"
)

type AccountStorage struct {
	db     *sql.DB
	logger *zap.Logger
	mu     sync.Mutex
}

func NewAccountStorage(db *sql.DB, logger *zap.Logger) (*AccountStorage, error) {
	return &AccountStorage{
		db:     db,
		logger: logger,
		mu:     sync.Mutex{},
	}, nil
}

func (s *AccountStorage) GetCurrentBalance(ctx context.Context, userID int64) (*model.Balance, error) {
	var current *float64
	var withdrawn *float64

	err := s.db.QueryRowContext(ctx,
		`SELECT 
			SUM(difference)::FLOAT8 / 100.0,
			(SUM(CASE WHEN difference < 0 THEN -difference ELSE 0 END)::FLOAT8 / 100.0)
		FROM public.t_account
		WHERE user_id = $1`,
		userID).Scan(&current, &withdrawn)
	if err != nil {
		return nil, err
	}

	balance := &model.Balance{
		Current:   0,
		Withdrawn: 0,
	}
	if current != nil {
		balance.Current = *current
	}
	if withdrawn != nil {
		balance.Withdrawn = *withdrawn
	}

	return balance, nil
}

func (s *AccountStorage) AddAccrual(ctx context.Context, orderNumber string, userID int64, accrual float64) error {
	cents := int64(floatutils.Round(accrual, 2) * 100)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO public.t_account (user_id, order_number, difference)
		VALUES ($1, $2, $3)`,
		userID, orderNumber, cents)
	return err
}

func (s *AccountStorage) Withdraw(ctx context.Context, userID int64, order string, sum float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cents := int64(floatutils.Round(sum, 2) * 100)

	var availableCents int64
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(difference), 0)
		FROM public.t_account
		WHERE user_id = $1`,
		userID).Scan(&availableCents)
	if err != nil {
		return err
	}

	if availableCents < cents {
		return repository.ErrInsufficientFunds
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO public.t_account (user_id, order_number, difference)
		VALUES ($1, $2, $3)`,
		userID, order, -cents)
	return err
}

func (s *AccountStorage) GetWithdrawals(ctx context.Context, userID int64) ([]model.Withdrawal, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT 
			order_number,
			(-difference::FLOAT8 / 100.0),
			created_at
		FROM public.t_account
		WHERE user_id = $1 AND difference < 0
		ORDER BY created_at DESC`,
		userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var withdrawals []model.Withdrawal
	for rows.Next() {
		var w model.Withdrawal
		err = rows.Scan(&w.Order, &w.Sum, &w.ProcessedAt)
		if err != nil {
			return nil, err
		}
		withdrawals = append(withdrawals, w)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return withdrawals, nil
}

func (s *AccountStorage) IncreaseBalance(ctx context.Context, userID int64, orderNumber string, amount float64) error {
	amountInCents := int64(amount * 100)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO public.t_account (user_id, order_number, difference)
		VALUES ($1, $2, $3)`,
		userID, orderNumber, amountInCents)
	return err
}
