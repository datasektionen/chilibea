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
	sseEvents    chan fridgeData
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
		sseEvents:    make(chan fridgeData, 2),
	}

	// Set up HTTP server
	http.HandleFunc("GET /", s.redirectToAbout)
	http.HandleFunc("GET /meta", s.metaPage)
	http.HandleFunc("GET /about", s.aboutPage)
	http.HandleFunc("GET /laskkyl", s.sodaPage)
	http.HandleFunc("GET /admin", s.adminPage)
	http.HandleFunc("PUT /admin/fridge/{t}/add", s.addFridgeItem)
	http.HandleFunc("POST /admin/fridge/{n}/edit", s.editFridgeItem)
	http.HandleFunc("GET /admin/fridge/{n}/cancel", s.cancelFridgeEdit)
	http.HandleFunc("DELETE /admin/fridge/{n}/remove", s.removeFridgeItem)
	http.HandleFunc("POST /admin/fridge/{n}/remove", s.confirmDeleteFridgeItem)
	http.HandleFunc("POST /admin/fridge/{n}/save", s.saveFridgeItemEdit)
	http.HandleFunc("GET /sse/fridge", s.sseFridge)
	http.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.HandleFunc("GET /oidc/callback", s.HandleOAuth2)

	address := fmt.Sprintf(":%d", port)
	slog.Info("Bea started", "address", address)
	http.ListenAndServe(address, nil)
}

type Member struct {
	Name   string
	Email  string
	ImgUrl string
}

type Group struct {
	Name    string
	Desc    string
	Email   string
	Members []Member
}

type IndexData struct {
	Groups []Group
}

func (s *Service) aboutPage(w http.ResponseWriter, r *http.Request) {
	groups, err := getHiveGroup()
	if err != nil {
		slog.Error("Failed to get Hive groups", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := IndexData{
		Groups: groups,
	}

	w.Header().Set("Cache-Control", "public, max-age=86400")

	if err := s.t.ExecuteTemplate(w, "index.html", data); err != nil {
		slog.Error("Failed to execute template", "error", err)
	}
}

func (s *Service) redirectToAbout(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/about", http.StatusSeeOther)
}
