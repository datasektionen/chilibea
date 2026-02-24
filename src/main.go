package main

import (
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"strconv"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"
)

type Service struct {
	db           *pgxpool.Pool
	oauth2Config oauth2.Config
	verifier     *oidc.IDTokenVerifier
	ctx          context.Context
	t            *template.Template
}

func main() {
	port, err := strconv.Atoi(os.Getenv("PORT"))
	if err != nil {
		panic(err)
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		panic("DATABASE_URL environment variable is not set")
	}
	db, err := pgxpool.New(context.Background(), dbURL)
	if _, err := db.Exec(context.Background(), `
		CREATE TYPE fridgeType AS ENUM ('soda', 'snack');
		CREATE TABLE IF NOT EXISTS sodaFridge (name TEXT PRIMARY KEY, type fridgeType NOT NULL, price FLOAT NOT NULL);
		CREATE TABLE IF NOT EXISTS cleanPoints (id SERIAL PRIMARY KEY, kthid VARCHAR(10) NOT NULL, date TIMESTAMP NOT NULL);
		`); err != nil {
		slog.Error("Failed to create tables:", err)
	}

	tmpl, err := template.ParseGlob("**/*.html")
	if err != nil {
		slog.Error("Failed to parse templates:", err)
	}

	ctx := context.Background()
	oauth2Config, verifier := InitOIDC(ctx)

	s := Service{
		db:           db,
		oauth2Config: oauth2Config,
		verifier:     verifier,
		ctx:          ctx,
		t:            tmpl,
	}

	// Set up HTTP server
	http.HandleFunc("GET /", s.redirectToAbout)
	http.HandleFunc("GET /about", s.aboutPage)
	http.HandleFunc("GET /laskkyl", s.sodaPage)
	http.HandleFunc("GET /admin", s.adminPage)
	http.HandleFunc("PUT /admin/fridge/{t}/add", s.addFridgeItem)
	http.HandleFunc("POST /admin/fridge/{n}/edit", s.editFridgeItem)
	http.HandleFunc("GET /admin/fridge/{n}/cancel", s.cancelFridgeEdit)
	http.HandleFunc("DELETE /admin/fridge/{n}/remove", s.removeFridgeItem)
	http.HandleFunc("POST /admin/fridge/{n}/remove", s.confirmDeleteFridgeItem)
	http.HandleFunc("POST /admin/fridge/{n}/save", s.saveFridgeItemEdit)
	// http.HandleFunc("GET /api/", s.api)
	http.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.HandleFunc("GET /oidc/callback", s.HandleOAuth2)

	address := fmt.Sprintf(":%d", port)
	slog.Info("Bea started", "address", address)
	http.ListenAndServe(address, nil)
}

func (s *Service) aboutPage(w http.ResponseWriter, r *http.Request) {
	if err := s.t.ExecuteTemplate(w, "index.html", nil); err != nil {
		slog.Error("Failed to execute template:", err)
	}
}

func (s *Service) redirectToAbout(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/about", http.StatusSeeOther)
}
