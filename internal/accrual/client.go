package accrual

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/MarkMiraclee/gophermart/internal/models"
	"github.com/MarkMiraclee/gophermart/internal/storage"
	"github.com/go-resty/resty/v2"
	"github.com/sirupsen/logrus"
)

const (
	updateInterval = 1 * time.Second
)

type Client struct {
	address string
	storage storage.Storage
	log     *logrus.Logger
	client  *resty.Client
}

func NewClient(address string, s storage.Storage, log *logrus.Logger) *Client {
	return &Client{
		address: address,
		storage: s,
		log:     log,
		client:  resty.New(),
	}
}

func (c *Client) Start(ctx context.Context) {
	ticker := time.NewTicker(updateInterval)
	defer ticker.Stop()

	c.log.Info("accrual client started")

	for {
		select {
		case <-ctx.Done():
			c.log.Info("accrual client stopped")
			return
		case <-ticker.C:
			c.processOrders(ctx)
		}
	}
}

func (c *Client) processOrders(ctx context.Context) {
	orders, err := c.storage.GetOrdersByStatus(ctx, []string{"NEW", "PROCESSING"})
	if err != nil {
		c.log.Errorf("failed to get orders for processing: %v", err)
		return
	}

	for _, order := range orders {
		go c.updateOrderStatus(ctx, order.Number)
	}
}

func (c *Client) updateOrderStatus(ctx context.Context, orderNumber string) {
	url := fmt.Sprintf("%s/api/orders/%s", c.address, orderNumber)
	resp, err := c.client.R().SetContext(ctx).Get(url)
	if err != nil {
		c.log.Errorf("failed to request accrual for order %s: %v", orderNumber, err)
		return
	}

	switch resp.StatusCode() {
	case http.StatusOK:
		var accrualResp models.AccrualResponse
		if err := json.Unmarshal(resp.Body(), &accrualResp); err != nil {
			c.log.Errorf("failed to unmarshal accrual response for order %s: %v", orderNumber, err)
			return
		}
		if err := c.storage.UpdateOrder(ctx, accrualResp.Order, accrualResp.Status, accrualResp.Accrual); err != nil {
			c.log.Errorf("failed to update order %s: %v", orderNumber, err)
		}
	case http.StatusNoContent:
	case http.StatusTooManyRequests:
		retryAfter := resp.Header().Get("Retry-After")
		if seconds, err := strconv.Atoi(retryAfter); err == nil {
			c.log.Warnf("rate limit hit, sleeping for %d seconds", seconds)
			time.Sleep(time.Duration(seconds) * time.Second)
			c.updateOrderStatus(ctx, orderNumber)
		}
	}
}
