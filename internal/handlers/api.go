package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/MarkMiraclee/gophermart/internal/auth"
	"github.com/MarkMiraclee/gophermart/internal/luhn"
	"github.com/MarkMiraclee/gophermart/internal/middlewares"
	"github.com/MarkMiraclee/gophermart/internal/models"
	"github.com/MarkMiraclee/gophermart/internal/storage"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
)

const (
	jwtLifetime = 24 * time.Hour
)

type API struct {
	storage   storage.Storage
	log       *logrus.Logger
	jwtSecret string
}

func NewAPI(s storage.Storage, log *logrus.Logger, jwtSecret string) *API {
	return &API{
		storage:   s,
		log:       log,
		jwtSecret: jwtSecret,
	}
}

func (a *API) Register(w http.ResponseWriter, r *http.Request) {
	var req models.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request format", http.StatusBadRequest)
		return
	}

	if req.Login == "" || req.Password == "" {
		http.Error(w, "login and password must not be empty", http.StatusBadRequest)
		return
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		a.log.Errorf("failed to hash password: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	user, err := a.storage.CreateUser(r.Context(), req.Login, string(passwordHash))
	if err != nil {
		if errors.Is(err, storage.ErrLoginExists) {
			http.Error(w, "login already exists", http.StatusConflict)
			return
		}
		a.log.Errorf("failed to create user: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	token, err := auth.BuildJWTString(user.ID, a.jwtSecret, jwtLifetime)
	if err != nil {
		a.log.Errorf("failed to build JWT: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Authorization", "Bearer "+token)
	w.WriteHeader(http.StatusOK)
}

func (a *API) Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request format", http.StatusBadRequest)
		return
	}

	user, err := a.storage.GetUserByLogin(r.Context(), req.Login)
	if err != nil {
		a.log.Errorf("failed to get user: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if user == nil {
		http.Error(w, "invalid login/password pair", http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		http.Error(w, "invalid login/password pair", http.StatusUnauthorized)
		return
	}

	token, err := auth.BuildJWTString(user.ID, a.jwtSecret, jwtLifetime)
	if err != nil {
		a.log.Errorf("failed to build JWT: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Authorization", "Bearer "+token)
	w.WriteHeader(http.StatusOK)
}

func (a *API) CreateOrder(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middlewares.UserIDKey).(string)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusInternalServerError)
		return
	}
	orderNumber := string(body)

	if !luhn.IsValid(orderNumber) {
		http.Error(w, "invalid order number format", http.StatusUnprocessableEntity)
		return
	}

	err = a.storage.CreateOrder(r.Context(), userID, orderNumber)
	if err != nil {
		if errors.Is(err, storage.ErrOrderExists) {
			w.WriteHeader(http.StatusOK)
			return
		}
		if errors.Is(err, storage.ErrOrderExistsOther) {
			http.Error(w, "order already uploaded by another user", http.StatusConflict)
			return
		}
		a.log.Errorf("failed to create order: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (a *API) GetOrders(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middlewares.UserIDKey).(string)

	orders, err := a.storage.GetOrdersByUser(r.Context(), userID)
	if err != nil {
		a.log.Errorf("failed to get orders: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if len(orders) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(orders); err != nil {
		a.log.Errorf("failed to encode orders: %v", err)
	}
}

func (a *API) GetBalance(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middlewares.UserIDKey).(string)

	balance, err := a.storage.GetBalance(r.Context(), userID)
	if err != nil {
		a.log.Errorf("failed to get balance: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(balance); err != nil {
		a.log.Errorf("failed to encode balance: %v", err)
	}
}

func (a *API) Withdraw(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middlewares.UserIDKey).(string)

	var req models.WithdrawRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request format", http.StatusBadRequest)
		return
	}

	if !luhn.IsValid(req.Order) {
		http.Error(w, "invalid order number format", http.StatusUnprocessableEntity)
		return
	}

	err := a.storage.CreateWithdrawal(r.Context(), userID, req.Order, req.Sum)
	if err != nil {
		if errors.Is(err, storage.ErrInsufficientFunds) {
			http.Error(w, "insufficient funds", http.StatusPaymentRequired)
			return
		}
		a.log.Errorf("failed to create withdrawal: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (a *API) GetWithdrawals(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middlewares.UserIDKey).(string)

	withdrawals, err := a.storage.GetWithdrawalsByUser(r.Context(), userID)
	if err != nil {
		a.log.Errorf("failed to get withdrawals: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if len(withdrawals) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(withdrawals); err != nil {
		a.log.Errorf("failed to encode withdrawals: %v", err)
	}
}
