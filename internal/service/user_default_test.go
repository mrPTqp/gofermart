package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/mrPTqp/gofermart/internal/repository"
	"github.com/mrPTqp/gofermart/internal/model"
	"golang.org/x/crypto/bcrypt"
)

type mockUserRepository struct {
	users map[string]model.User
	err   error
}

func (m *mockUserRepository) FindByLogin(ctx context.Context, login string) (*model.User, error) {
	if m.err != nil {
		return nil, m.err
	}
	user, ok := m.users[login]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &user, nil
}

func (m *mockUserRepository) Create(ctx context.Context, login, passwordHash string) error {
	if m.err != nil {
		return m.err
	}

	id := int64(len(m.users) + 1)
	m.users[login] = model.User{
		ID:             id,
		Login:          login,
		PasswordHash:   passwordHash,
	}
	return nil
}

func TestUserService_Register(t *testing.T) {
	tests := []struct {
		name           string
		login          string
		password       string
		mockUsers      map[string]model.User
		mockErr        error
		wantErr        bool
		wantToken      bool
		wantUserID     bool
	}{
		{
			name:           "successful registration",
			login:          "newuser",
			password:       "password123",
			mockUsers:      map[string]model.User{},
			mockErr:        nil,
			wantErr:        false,
			wantToken:      true,
			wantUserID:     true,
		},
		{
			name:           "user already exists",
			login:          "existing",
			password:       "password123",
			mockUsers:      map[string]model.User{"existing": {}},
			mockErr:        nil,
			wantErr:        true,
			wantToken:      false,
			wantUserID:     false,
		},
		{
			name:           "repo error on check",
			login:          "user",
			password:       "password123",
			mockUsers:      nil,
			mockErr:        errors.New("db error"),
			wantErr:        true,
			wantToken:      false,
			wantUserID:     false,
		},
		{
			name:           "empty login",
			login:          "",
			password:       "password123",
			mockUsers:      map[string]model.User{},
			wantErr:        true,
			wantToken:      false,
			wantUserID:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockUserRepository{users: tt.mockUsers, err: tt.mockErr}
			svc := NewUserService(repo, "secret", time.Hour)

			userID, token, err := svc.Register(context.Background(), tt.login, tt.password)

			if (err != nil) != tt.wantErr {
				t.Fatalf("Register() error = %v, wantErr = %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if userID == 0 {
					t.Error("Expected non-zero userID")
				}
				if tt.wantToken && token == "" {
					t.Error("Expected non-empty token")
				}
				if _, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
					return []byte("secret"), nil
				}); err != nil {
					t.Error("Token is invalid:", err)
				}
			}
		})
	}
}

func TestUserService_Login(t *testing.T) {
    hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)

    tests := []struct {
        name           string
        login          string
        password       string
        mockUsers      map[string]model.User
        mockErr        error
        wantErr        bool
        wantToken      bool
    }{
        {
            name: "valid credentials",
            login: "user",
            password: "password123",
            mockUsers: map[string]model.User{
                "user": {
                    ID:           1,
                    Login:        "user",
                    PasswordHash: string(hash),
                },
            },
            wantErr:   false,
            wantToken: true,
        },
        {
            name: "wrong password",
            login: "user",
            password: "wrong",
            mockUsers: map[string]model.User{
                "user": {
                    ID:           1,
                    Login:        "user",
                    PasswordHash: string(hash),
                },
            },
            wantErr:   true,
            wantToken: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            repo := &mockUserRepository{users: tt.mockUsers, err: tt.mockErr}
            svc := NewUserService(repo, "secret", time.Hour)

            userID, token, err := svc.Login(context.Background(), tt.login, tt.password)

            if (err != nil) != tt.wantErr {
                t.Fatalf("Login() error = %v, wantErr = %v", err, tt.wantErr)
            }
            if !tt.wantErr {
                if userID == 0 {
                    t.Error("Expected non-zero userID")
                }
                if !tt.wantToken {
                    t.Error("Expected token")
                }
                if _, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
                    return []byte("secret"), nil
                }); err != nil {
                    t.Error("Token is invalid:", err)
                }
            }
        })
    }
}

