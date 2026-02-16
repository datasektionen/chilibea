package main

import (
	"context"
	"embed"
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
	oauth2Config *oauth2.Config
	verifier     *oidc.IDTokenVerifier
	ctx          context.Context
	t            *template.Template
}

//go:embed templates/*
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

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
	CREATE TABLE IF NOT EXISTS sodaFridge (id SERIAL PRIMARY KEY, name TEXT NOT NULL, price FLOAT NOT NULL);
	CREATE TABLE IF NOT EXISTS cleanPoints (id SERIAL PRIMARY KEY, kthid VARCHAR(10) NOT NULL, date TIMESTAMP NOT NULL);
		`); err != nil {
		slog.Error("Failed to create tables:", err)
	}

	tmpl, err := template.ParseFS(templatesFS, "**/*.html")
	if err != nil {
		slog.Error("Failed to parse templates:", err)
	}

	ctx := context.Background()
	oauth2Config, verifier := InitOIDC(ctx)

	s := Service{
		db:           db,
		oauth2Config: &oauth2Config,
		verifier:     verifier,
		ctx:          ctx,
		t:            tmpl,
	}

	// Set up HTTP server
	http.HandleFunc("GET /", s.redirectToAbout)
	http.HandleFunc("GET /about", s.aboutPage)
	// http.HandleFunc("GET /admin", s.adminPage)
	// http.HandleFunc("GET /api/", s.api)
	http.Handle("GET /static/", http.FileServerFS(staticFS))
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
