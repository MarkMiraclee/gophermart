package storage

import (
	"context"
	"errors"
	"time"

	"github.com/MarkMiraclee/gophermart/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

var (
	ErrLoginExists       = errors.New("login already exists")
	ErrOrderExists       = errors.New("order already exists for this user")
	ErrOrderExistsOther  = errors.New("order already exists for another user")
	ErrInsufficientFunds = errors.New("insufficient funds")
)

type PostgresStorage struct {
	pool *pgxpool.Pool
	log  *logrus.Logger
}

func NewPostgresStorage(ctx context.Context, dsn string, log *logrus.Logger) (*PostgresStorage, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}

	storage := &PostgresStorage{pool: pool, log: log}
	if err := storage.runMigrations(ctx); err != nil {
		return nil, err
	}

	return storage, nil
}

func (s *PostgresStorage) runMigrations(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY,
			login VARCHAR(255) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL
		);

		CREATE TABLE IF NOT EXISTS orders (
			id UUID PRIMARY KEY,
			user_id UUID REFERENCES users(id),
			number VARCHAR(255) UNIQUE NOT NULL,
			status VARCHAR(50) NOT NULL,
			accrual NUMERIC,
			uploaded_at TIMESTAMPTZ NOT NULL
		);

		CREATE TABLE IF NOT EXISTS withdrawals (
			id UUID PRIMARY KEY,
			user_id UUID REFERENCES users(id),
			order_number VARCHAR(255) NOT NULL,
			sum NUMERIC NOT NULL,
			processed_at TIMESTAMPTZ NOT NULL
		);
	`)
	return err
}

func (s *PostgresStorage) Close() {
	s.pool.Close()
}

func (s *PostgresStorage) CreateUser(ctx context.Context, login, passwordHash string) (*models.User, error) {
	user := &models.User{
		ID:           uuid.NewString(),
		Login:        login,
		PasswordHash: passwordHash,
	}
	_, err := s.pool.Exec(ctx, "INSERT INTO users (id, login, password_hash) VALUES ($1, $2, $3)", user.ID, user.Login, user.PasswordHash)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			return nil, ErrLoginExists
		}
		return nil, err
	}
	return user, nil
}

func (s *PostgresStorage) GetUserByLogin(ctx context.Context, login string) (*models.User, error) {
	user := &models.User{}
	err := s.pool.QueryRow(ctx, "SELECT id, login, password_hash FROM users WHERE login = $1", login).Scan(&user.ID, &user.Login, &user.PasswordHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // User not found
		}
		return nil, err
	}
	return user, nil
}

func (s *PostgresStorage) CreateOrder(ctx context.Context, userID, orderNumber string) error {
	_, err := s.pool.Exec(ctx, "INSERT INTO orders (id, user_id, number, status, uploaded_at) VALUES ($1, $2, $3, $4, $5)",
		uuid.NewString(), userID, orderNumber, "NEW", time.Now())
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			existingOrder, getErr := s.GetOrderByNumber(ctx, orderNumber)
			if getErr != nil {
				return getErr
			}
			if existingOrder.UserID == userID {
				return ErrOrderExists
			}
			return ErrOrderExistsOther
		}
		return err
	}
	return nil
}

func (s *PostgresStorage) GetOrderByNumber(ctx context.Context, orderNumber string) (*models.Order, error) {
	order := &models.Order{}
	err := s.pool.QueryRow(ctx, "SELECT user_id, number, status, accrual, uploaded_at FROM orders WHERE number = $1", orderNumber).
		Scan(&order.UserID, &order.Number, &order.Status, &order.Accrual, &order.UploadedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, pgx.ErrNoRows
		}
		return nil, err
	}
	return order, nil
}

func (s *PostgresStorage) GetOrdersByUser(ctx context.Context, userID string) ([]models.Order, error) {
	rows, err := s.pool.Query(ctx, "SELECT number, status, accrual, uploaded_at FROM orders WHERE user_id = $1 ORDER BY uploaded_at DESC", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []models.Order
	for rows.Next() {
		var order models.Order
		if err := rows.Scan(&order.Number, &order.Status, &order.Accrual, &order.UploadedAt); err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, nil
}

func (s *PostgresStorage) GetOrdersByStatus(ctx context.Context, statuses []string) ([]models.Order, error) {
	query := "SELECT number, status, accrual, uploaded_at FROM orders WHERE status = ANY($1)"
	rows, err := s.pool.Query(ctx, query, statuses)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []models.Order
	for rows.Next() {
		var order models.Order
		if err := rows.Scan(&order.Number, &order.Status, &order.Accrual, &order.UploadedAt); err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, nil
}

func (s *PostgresStorage) UpdateOrder(ctx context.Context, orderNumber, status string, accrual *float64) error {
	_, err := s.pool.Exec(ctx, "UPDATE orders SET status = $1, accrual = $2 WHERE number = $3", status, accrual, orderNumber)
	return err
}

func (s *PostgresStorage) GetBalance(ctx context.Context, userID string) (*models.Balance, error) {
	balance := &models.Balance{}

	err := s.pool.QueryRow(ctx, "SELECT COALESCE(SUM(accrual), 0) FROM orders WHERE user_id = $1 AND status = 'PROCESSED'", userID).Scan(&balance.Current)
	if err != nil {
		return nil, err
	}

	err = s.pool.QueryRow(ctx, "SELECT COALESCE(SUM(sum), 0) FROM withdrawals WHERE user_id = $1", userID).Scan(&balance.Withdrawn)
	if err != nil {
		return nil, err
	}

	balance.Current -= balance.Withdrawn

	return balance, nil
}

func (s *PostgresStorage) CreateWithdrawal(ctx context.Context, userID, orderNumber string, sum float64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			s.log.Errorf("failed to rollback transaction: %v", err)
		}
	}()
	var balance models.Balance
	err = tx.QueryRow(ctx, "SELECT COALESCE(SUM(accrual), 0) FROM orders WHERE user_id = $1 AND status = 'PROCESSED'", userID).Scan(&balance.Current)
	if err != nil {
		return err
	}

	err = tx.QueryRow(ctx, "SELECT COALESCE(SUM(sum), 0) FROM withdrawals WHERE user_id = $1", userID).Scan(&balance.Withdrawn)
	if err != nil {
		return err
	}

	if (balance.Current - balance.Withdrawn) < sum {
		return ErrInsufficientFunds
	}

	_, err = tx.Exec(ctx, "INSERT INTO withdrawals (id, user_id, order_number, sum, processed_at) VALUES ($1, $2, $3, $4, $5)",
		uuid.NewString(), userID, orderNumber, sum, time.Now())
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *PostgresStorage) GetWithdrawalsByUser(ctx context.Context, userID string) ([]models.Withdrawal, error) {
	rows, err := s.pool.Query(ctx, "SELECT order_number, sum, processed_at FROM withdrawals WHERE user_id = $1 ORDER BY processed_at DESC", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var withdrawals []models.Withdrawal
	for rows.Next() {
		var w models.Withdrawal
		if err := rows.Scan(&w.OrderNumber, &w.Sum, &w.ProcessedAt); err != nil {
			return nil, err
		}
		withdrawals = append(withdrawals, w)
	}
	return withdrawals, nil
}
