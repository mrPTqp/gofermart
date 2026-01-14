// internal/storage/postgres/order.go
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/mrPTqp/gofermart/internal/model"
	"github.com/mrPTqp/gofermart/internal/repository"
	"go.uber.org/zap"
)

type OrderStorage struct {
	db     *sql.DB
	logger *zap.SugaredLogger
}

func NewOrderStorage(db *sql.DB, logger *zap.SugaredLogger) (*OrderStorage, error) {
	return &OrderStorage{db: db, logger: logger}, nil
}

func (s *OrderStorage) Create(ctx context.Context, number string, userID int64) error {
	var existingUserID int64
	err := s.db.QueryRowContext(ctx,
		`SELECT user_id FROM public.t_order WHERE number = $1`,
		number).Scan(&existingUserID)

	if err == nil && existingUserID != userID {
		return repository.ErrOrderTaken
	}
	if err == nil && existingUserID == userID {
		return repository.ErrOrderExists
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO public.t_order (number, user_id) VALUES ($1, $2)`,
		number, userID)
	return err
}

func (s *OrderStorage) GetByUser(ctx context.Context, userID int64) ([]model.Order, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT 
			o.number, 
			o.created_at, 
			o.updated_at, 
			o.status_code, 
			o.last_checked_at,
			(a.difference::FLOAT8 / 100.0) AS accrual
		FROM public.t_order o
		LEFT JOIN public.t_account a ON a.order_number = o.number AND a.difference > 0
		WHERE o.user_id = $1
		ORDER BY o.created_at DESC`,
		userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []model.Order
	for rows.Next() {
		var o model.Order
		var lastChecked sql.NullTime
		var accrual *float64

		err = rows.Scan(
			&o.Number,
			&o.CreatedAt,
			&o.UpdatedAt,
			&o.StatusCode,
			&lastChecked,
			&accrual,
		)
		if err != nil {
			return nil, err
		}

		if lastChecked.Valid {
			o.LastCheckedAt = lastChecked.Time
		} else {
			o.LastCheckedAt = time.Time{}
		}

		o.Accrual = accrual
		orders = append(orders, o)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return orders, nil
}

func (s *OrderStorage) GetByNumber(ctx context.Context, number string) (*model.Order, error) {
	var order model.Order
	var lastChecked sql.NullTime

	err := s.db.QueryRowContext(ctx,
		`SELECT 
			number, 
			user_id, 
			created_at, 
			updated_at, 
			status_code, 
			last_checked_at
		FROM public.t_order 
		WHERE number = $1`,
		number).Scan(
		&order.Number,
		&order.UserID,
		&order.CreatedAt,
		&order.UpdatedAt,
		&order.StatusCode,
		&lastChecked,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}

	if lastChecked.Valid {
		order.LastCheckedAt = lastChecked.Time
	} else {
		order.LastCheckedAt = time.Time{}
	}

	return &order, nil
}

func (s *OrderStorage) UpdateStatus(ctx context.Context, number, status string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE public.t_order
		SET status_code = $1, last_checked_at = NOW(), updated_at = NOW()
		WHERE number = $2`,
		status, number)
	return err
}

func (s *OrderStorage) GetOrdersForProcessing(ctx context.Context) ([]model.Order, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT 
			id,
			number, 
			user_id, 
			created_at, 
			updated_at, 
			status_code, 
			last_checked_at,
			accrual
		FROM public.t_order 
		WHERE status_code IN ('NEW', 'PROCESSING')
		ORDER BY created_at ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []model.Order
	for rows.Next() {
		var order model.Order
		var lastChecked sql.NullTime
		var accrual *float64

		err = rows.Scan(
			&order.ID,
			&order.Number,
			&order.UserID,
			&order.CreatedAt,
			&order.UpdatedAt,
			&order.StatusCode,
			&lastChecked,
			&accrual,
		)
		if err != nil {
			return nil, err
		}

		if lastChecked.Valid {
			order.LastCheckedAt = lastChecked.Time
		} else {
			order.LastCheckedAt = time.Time{}
		}
		order.Accrual = accrual
		orders = append(orders, order)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return orders, nil
}

func (s *OrderStorage) Update(ctx context.Context, order *model.Order) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE public.t_order
		SET status_code = $1, 
			accrual = $2, 
			last_checked_at = NOW(), 
			updated_at = NOW()
		WHERE number = $3`,
		string(order.StatusCode),
		order.Accrual,
		order.Number,
	)
	return err
}
