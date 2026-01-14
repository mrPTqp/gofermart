// internal/storage/postgres/user.go
package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgerrcode"
	"github.com/mrPTqp/gofermart/internal/model"
	"github.com/mrPTqp/gofermart/internal/repository"
	"go.uber.org/zap"
)

type UserStorage struct {
	db     *sql.DB
	logger *zap.SugaredLogger
}

func NewUserStorage(db *sql.DB, logger *zap.SugaredLogger) (*UserStorage, error) {
	return &UserStorage{db: db, logger: logger}, nil
}

func (s *UserStorage) Create(ctx context.Context, login, passwordHash string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO public.t_user (login, password_hash) VALUES ($1, $2)`,
		login, passwordHash)

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
		return repository.ErrLoginExists
	}

	return err
}

func (s *UserStorage) FindByLogin(ctx context.Context, login string) (*model.User, error) {
	var user model.User
	err := s.db.QueryRowContext(ctx,
		`SELECT id, login, password_hash FROM public.t_user WHERE login = $1`,
		login).Scan(&user.ID, &user.Login, &user.PasswordHash)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}

	return &user, nil
}
