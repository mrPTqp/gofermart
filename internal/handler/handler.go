package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt"
	"github.com/mrPTqp/gofermart/internal/config"
	"github.com/mrPTqp/gofermart/internal/luhn"
	"github.com/mrPTqp/gofermart/internal/repository"
	"github.com/mrPTqp/gofermart/internal/service"
	"go.uber.org/zap"
)

func decodeJSON[T any](r *http.Request) (*T, error) {
	if r.Header.Get("Content-Type") != "application/json" {
		return nil, errors.New("content-type must be application/json")
	}
	defer r.Body.Close()

	var dst T
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&dst); err != nil {
		return nil, err
	}
	return &dst, nil
}

type AccrualResponse struct {
	Order   string   `json:"order"`
	Status  string   `json:"status"`
	Accrual *float64 `json:"accrual,omitempty"`
}

type UserIDKey string

const UserIDKeyContext UserIDKey = "userID"

type UserClaims struct {
	UserID int64 `json:"user_id"`
	jwt.StandardClaims
}

type GofermartHandler struct {
	UserService    service.UserService
	OrderService   service.OrderService
	AccountService service.AccountService
	Config         *config.Config
	Logger         *zap.Logger
}

func NewGofermartHandler(
	userService service.UserService,
	orderService service.OrderService,
	accountService service.AccountService,
	config *config.Config,
	logger *zap.Logger,
) *GofermartHandler {
	return &GofermartHandler{
		UserService:    userService,
		OrderService:   orderService,
		AccountService: accountService,
		Config:         config,
		Logger:         logger,
	}
}

func (h *GofermartHandler) getUID(r *http.Request) (int64, error) {
	uid, ok := r.Context().Value(UserIDKeyContext).(int64)
	if !ok {
		return 0, errors.New("user not authenticated")
	}
	return uid, nil
}

func (h *GofermartHandler) Register(w http.ResponseWriter, r *http.Request) {
	input, err := decodeJSON[struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}](r)
	if err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if input.Login == "" || input.Password == "" {
		http.Error(w, "login and password required", http.StatusBadRequest)
		return
	}

	_, token, err := h.UserService.Register(r.Context(), input.Login, input.Password)
	if err != nil {
		if errors.Is(err, repository.ErrLoginExists) {
			http.Error(w, "user already exists", http.StatusConflict)
			return
		}
		h.Logger.Error("register error", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Authorization", "Bearer "+token)
	w.WriteHeader(http.StatusOK)
}

func (h *GofermartHandler) Login(w http.ResponseWriter, r *http.Request) {
	input, err := decodeJSON[struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}](r)
	if err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	_, token, err := h.UserService.Login(r.Context(), input.Login, input.Password)
	if err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Authorization", "Bearer "+token)
	w.WriteHeader(http.StatusOK)
}

func (h *GofermartHandler) UploadOrder(w http.ResponseWriter, r *http.Request) {
	userID, err := h.getUID(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	orderNum := strings.TrimSpace(string(body))
	if orderNum == "" {
		http.Error(w, "empty order number", http.StatusBadRequest)
		return
	}

	if !luhn.IsValid(orderNum) {
		http.Error(w, "invalid order number (Luhn)", http.StatusUnprocessableEntity)
		return
	}

	err = h.OrderService.UploadOrder(r.Context(), userID, orderNum)
	switch {
	case err == nil:
		w.WriteHeader(http.StatusAccepted)
		return
	case errors.Is(err, repository.ErrOrderExists):
		w.WriteHeader(http.StatusOK)
		return
	case errors.Is(err, repository.ErrAnotherUser):
		http.Error(w, "order belongs to another user", http.StatusConflict)
		return
	default:
		h.Logger.Error("upload order error", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
}

func (h *GofermartHandler) GetOrders(w http.ResponseWriter, r *http.Request) {
	userID, err := h.getUID(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	orders, err := h.OrderService.GetUserOrders(r.Context(), userID)
	if err != nil {
		h.Logger.Error("get orders error", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if len(orders) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(orders)
}

func (h *GofermartHandler) GetBalance(w http.ResponseWriter, r *http.Request) {
	userID, err := h.getUID(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	balance, err := h.AccountService.GetBalance(r.Context(), userID)
	if err != nil {
		h.Logger.Error("get balance error", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(balance)
}

func (h *GofermartHandler) WithdrawBalance(w http.ResponseWriter, r *http.Request) {
	userID, err := h.getUID(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	input, err := decodeJSON[struct {
		Order string  `json:"order"`
		Sum   float64 `json:"sum"`
	}](r)
	if err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if input.Order == "" || input.Sum <= 0 {
		http.Error(w, "order and positive sum required", http.StatusBadRequest)
		return
	}
	if !luhn.IsValid(input.Order) {
		http.Error(w, "invalid order number (Luhn)", http.StatusUnprocessableEntity)
		return
	}

	err = h.AccountService.Withdraw(r.Context(), userID, input.Order, input.Sum)
	if err != nil {
		if err.Error() == "insufficient funds" {
			http.Error(w, "insufficient funds", http.StatusPaymentRequired)
			return
		}
		h.Logger.Error("withdraw error", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *GofermartHandler) GetWithdrawals(w http.ResponseWriter, r *http.Request) {
	userID, err := h.getUID(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	withdrawals, err := h.AccountService.GetWithdrawals(r.Context(), userID)
	if err != nil {
		h.Logger.Error("get withdrawals error", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if len(withdrawals) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(withdrawals)
}
