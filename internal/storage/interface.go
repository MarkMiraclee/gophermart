package storage

import (
	"context"
	"github.com/MarkMiraclee/gophermart/internal/models"
)

type Storage interface {
	CreateUser(ctx context.Context, login, passwordHash string) (*models.User, error)
	GetUserByLogin(ctx context.Context, login string) (*models.User, error)

	CreateOrder(ctx context.Context, userID, orderNumber string) error
	GetOrderByNumber(ctx context.Context, orderNumber string) (*models.Order, error)
	GetOrdersByUser(ctx context.Context, userID string) ([]models.Order, error)
	GetOrdersByStatus(ctx context.Context, statuses []string) ([]models.Order, error)
	UpdateOrder(ctx context.Context, orderNumber, status string, accrual *float64) error

	GetBalance(ctx context.Context, userID string) (*models.Balance, error)
	CreateWithdrawal(ctx context.Context, userID, orderNumber string, sum float64) error
	GetWithdrawalsByUser(ctx context.Context, userID string) ([]models.Withdrawal, error)

	Close()
}
