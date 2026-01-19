package postgres

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jackc/pgconn"
	"github.com/mrPTqp/gofermart/internal/repository"
	"go.uber.org/zap/zaptest"
)

func newUserStorageWithMockDB(t *testing.T) (*UserStorage, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}

	logger := zaptest.NewLogger(t)
	storage, err := NewUserStorage(db, logger)
	if err != nil {
		t.Fatalf("failed to create UserStorage: %v", err)
	}

	return storage, mock
}

func TestUserStorage_Create(t *testing.T) {
	storage, mock := newUserStorageWithMockDB(t)
	defer mock.ExpectClose()

	tests := []struct {
		name           string
		login          string
		passwordHash   string
		mockSetup      func()
		expectedError  error
	}{
		{
			name:         "successful creation",
			login:        "user1",
			passwordHash: "hash1",
			mockSetup: func() {
				mock.ExpectExec("INSERT INTO public.t_user").
					WithArgs("user1", "hash1").
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			expectedError: nil,
		},
		{
			name:         "duplicate login",
			login:        "user1",
			passwordHash: "hash2",
			mockSetup: func() {
				pgErr := &pgconn.PgError{
					Code: "23505", // unique_violation
				}
				mock.ExpectExec("INSERT INTO public.t_user").
					WithArgs("user1", "hash2").
					WillReturnError(pgErr)
			},
			expectedError: repository.ErrLoginExists,
		},
		{
			name:         "other db error",
			login:        "user1",
			passwordHash: "hash3",
			mockSetup: func() {
				mock.ExpectExec("INSERT INTO public.t_user").
					WithArgs("user1", "hash3").
					WillReturnError(errors.New("random db error"))
			},
			expectedError: errors.New("random db error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			err := storage.Create(context.Background(), tt.login, tt.passwordHash)

			if tt.expectedError == nil {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.expectedError)
				} else if tt.expectedError != repository.ErrLoginExists && err.Error() != tt.expectedError.Error() {
					t.Errorf("expected %v, got %v", tt.expectedError, err)
				}
				if tt.expectedError == repository.ErrLoginExists && err != repository.ErrLoginExists {
					t.Errorf("expected ErrLoginExists, got %v", err)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestUserStorage_FindByLogin(t *testing.T) {
	storage, mock := newUserStorageWithMockDB(t)
	defer mock.ExpectClose()

	tests := []struct {
		name           string
		login          string
		mockSetup      func()
		expectedUser   *string
		expectedError  error
	}{
		{
			name:  "user found",
			login: "testuser",
			mockSetup: func() {
				rows := sqlmock.NewRows([]string{"id", "login", "password_hash"}).
					AddRow(1, "testuser", "hash123")
				mock.ExpectQuery("SELECT id, login, password_hash FROM public.t_user").
					WithArgs("testuser").
					WillReturnRows(rows)
			},
			expectedUser:  strPtr("testuser"),
			expectedError: nil,
		},
		{
			name:  "user not found",
			login: "unknown",
			mockSetup: func() {
				mock.ExpectQuery("SELECT id, login, password_hash FROM public.t_user").
					WithArgs("unknown").
					WillReturnError(sql.ErrNoRows)
			},
			expectedUser:  nil,
			expectedError: repository.ErrNotFound,
		},
		{
			name:  "db error",
			login: "broken",
			mockSetup: func() {
				mock.ExpectQuery("SELECT id, login, password_hash FROM public.t_user").
					WithArgs("broken").
					WillReturnError(errors.New("db connection lost"))
			},
			expectedUser:  nil,
			expectedError: errors.New("db connection lost"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			user, err := storage.FindByLogin(context.Background(), tt.login)

			if tt.expectedError != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.expectedError)
				} else if !errors.Is(err, tt.expectedError) && err.Error() != tt.expectedError.Error() {
					t.Errorf("expected %v, got %v", tt.expectedError, err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
				if user == nil {
					t.Errorf("expected user, got nil")
				} else if user.Login != *tt.expectedUser {
					t.Errorf("expected login %s, got %s", *tt.expectedUser, user.Login)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}
