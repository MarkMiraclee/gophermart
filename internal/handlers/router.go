package handlers

import (
	"github.com/MarkMiraclee/gophermart/internal/middlewares"
	"github.com/go-chi/chi/v5"
)

func NewRouter(api *API) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middlewares.Logger(api.log))
	r.Use(middlewares.Gzip)

	r.Route("/api/user", func(r chi.Router) {
		r.Post("/register", api.Register)
		r.Post("/login", api.Login)

		r.Group(func(r chi.Router) {
			r.Use(middlewares.Auth(api.jwtSecret))
			r.Post("/orders", api.CreateOrder)
			r.Get("/orders", api.GetOrders)
			r.Get("/balance", api.GetBalance)
			r.Post("/balance/withdraw", api.Withdraw)
			r.Get("/withdrawals", api.GetWithdrawals)
		})
	})
	return r
}
