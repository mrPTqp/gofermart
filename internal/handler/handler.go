// internal/handler/handler.go
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
	BalanceService service.AccountService
	Config         *config.Config
	Logger         *zap.SugaredLogger
}

func NewGofermartHandler(
	userService service.UserService,
	orderService service.OrderService,
	balanceService service.AccountService,
	config *config.Config,
	logger *zap.SugaredLogger,
) *GofermartHandler {
	return &GofermartHandler{
		UserService:    userService,
		OrderService:   orderService,
		BalanceService: balanceService,
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

func (h *GofermartHandler) decodeBody(r *http.Request, dst interface{}) error {
	defer r.Body.Close()
	if r.Header.Get("Content-Type") != "application/json" {
		return errors.New("content-type must be application/json")
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(dst)
}

func (h *GofermartHandler) Register(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	if err := h.decodeBody(r, &input); err != nil {
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
		h.Logger.Errorf("register error: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Authorization", "Bearer "+token)
	w.WriteHeader(http.StatusOK)
}

func (h *GofermartHandler) Login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	if err := h.decodeBody(r, &input); err != nil {
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
		http.Error(w, "invalid order number (Luhn)", http.StatusBadRequest)
		return
	}

	err = h.OrderService.UploadOrder(r.Context(), userID, orderNum)
	if err != nil {
		switch err.Error() {
		case "already uploaded":
			w.WriteHeader(http.StatusOK)
			return
		case "another user":
			http.Error(w, "order belongs to another user", http.StatusConflict)
			return
		default:
			h.Logger.Errorf("upload order error: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusAccepted)
}

func (h *GofermartHandler) GetOrders(w http.ResponseWriter, r *http.Request) {
	userID, err := h.getUID(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	orders, err := h.OrderService.GetUserOrders(r.Context(), userID)
	if err != nil {
		h.Logger.Errorf("get orders error: %v", err)
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

	balance, err := h.BalanceService.GetBalance(r.Context(), userID)
	if err != nil {
		h.Logger.Errorf("get balance error: %v", err)
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

	var input struct {
		Order string  `json:"order"`
		Sum   float64 `json:"sum"`
	}
	if err := h.decodeBody(r, &input); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if input.Order == "" || input.Sum <= 0 {
		http.Error(w, "order and positive sum required", http.StatusBadRequest)
		return
	}
	if !luhn.IsValid(input.Order) {
		http.Error(w, "invalid order number (Luhn)", http.StatusBadRequest)
		return
	}

	err = h.BalanceService.Withdraw(r.Context(), userID, input.Order, input.Sum)
	if err != nil {
		if err.Error() == "insufficient funds" {
			http.Error(w, "insufficient funds", http.StatusPaymentRequired)
			return
		}
		h.Logger.Errorf("withdraw error: %v", err)
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

	withdrawals, err := h.BalanceService.GetWithdrawals(r.Context(), userID)
	if err != nil {
		h.Logger.Errorf("get withdrawals error: %v", err)
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
