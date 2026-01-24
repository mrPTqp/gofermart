package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/golang-jwt/jwt"
	"github.com/mrPTqp/gofermart/internal/repository"
)

type UserClaims struct {
	UserID int64 `json:"user_id"`
	jwt.StandardClaims
}

type UserServiceDefault struct {
    repo       repository.UserRepository
    jwtSecret  []byte
    tokenTTL   time.Duration
}

func NewUserService(repo repository.UserRepository, jwtSecret string, ttl time.Duration) *UserServiceDefault {
    return &UserServiceDefault{
        repo:       repo,
        jwtSecret:  []byte(jwtSecret),
        tokenTTL:   ttl, 
    }
}

func (s *UserServiceDefault) Register(ctx context.Context, login, password string) (int64, string, error) {
	if login == "" || password == "" {
		return 0, "", errors.New("login and password required")
	}

	_, err := s.repo.FindByLogin(ctx, login)
	if err == nil {
		return 0, "", repository.ErrLoginExists
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return 0, "", fmt.Errorf("failed to check user existence: %w", err)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return 0, "", fmt.Errorf("failed to hash password: %w", err)
	}

	if err := s.repo.Create(ctx, login, string(hashedPassword)); err != nil {
		return 0, "", fmt.Errorf("failed to create user: %w", err)
	}

	user, err := s.repo.FindByLogin(ctx, login)
	if err != nil {
		return 0, "", fmt.Errorf("failed to fetch created user: %w", err)
	}

	token, err := s.generateJWT(user.ID)
	if err != nil {
		return 0, "", fmt.Errorf("failed to generate token: %w", err)
	}

	return user.ID, token, nil
}


func (s *UserServiceDefault) Login(ctx context.Context, login, password string) (int64, string, error) {
	user, err := s.repo.FindByLogin(ctx, login)
	if err != nil {
		return 0, "", errors.New("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return 0, "", errors.New("invalid credentials")
	}

	token, err := s.generateJWT(user.ID)
	if err != nil {
		return 0, "", fmt.Errorf("failed to generate token: %w", err)
	}

	return user.ID, token, nil
}

func (s *UserServiceDefault) generateJWT(userID int64) (string, error) {
    claims := &UserClaims{
        UserID: userID,
        StandardClaims: jwt.StandardClaims{
            ExpiresAt: time.Now().Add(s.tokenTTL).Unix(),
        },
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString(s.jwtSecret)
}
