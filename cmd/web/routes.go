package main

import (
	"GOSockets/internal/handlers"
	"github.com/bmizerany/pat"
	"net/http"
)

// routes defines the application routes
func routes() http.Handler {
	mux := pat.New()

	mux.Get("/", http.HandlerFunc(handlers.Home))

	return mux
}
