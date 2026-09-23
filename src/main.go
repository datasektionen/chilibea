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
	"github.com/jackc/pgx/v5/pgtype"
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
		`); err != nil {
		slog.Error("Enum type duplicate", err)
	}
	if _, err := db.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS cleanPoints (kthid VARCHAR(10), date DATE, PRIMARY KEY (date, kthid));
		CREATE TABLE IF NOT EXISTS sodaFridge (name TEXT PRIMARY KEY, type fridgeType NOT NULL, price FLOAT NOT NULL, priority INT NOT NULL);
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
	http.HandleFunc("GET /mandagsstad", s.mandagsstad)
	http.HandleFunc("GET /mandagsstad/all_time", s.mandagsstadAllTime)
	http.HandleFunc("GET /mandagsstad/year", s.mandagsstadYear)
	http.HandleFunc("GET /admin", s.adminPage)
	http.HandleFunc("PUT /admin/fridge/{t}/add", s.addFridgeItem)
	http.HandleFunc("POST /admin/fridge/{n}/edit", s.editFridgeItem)
	http.HandleFunc("GET /admin/fridge/{n}/cancel", s.cancelFridgeEdit)
	http.HandleFunc("DELETE /admin/fridge/{n}/remove", s.removeFridgeItem)
	http.HandleFunc("POST /admin/fridge/{n}/remove", s.confirmDeleteFridgeItem)
	http.HandleFunc("POST /admin/fridge/{n}/save", s.saveFridgeItemEdit)
	http.HandleFunc("POST /admin/fridge/priority", s.updateFridgeItemPriority)
	http.HandleFunc("GET /admin/cleaning/search/{id}", s.searchSSOusers)
	http.HandleFunc("PUT /admin/cleaning/point", s.addCleaningPoint)
	http.HandleFunc("GET /admin/cleaning/user", s.searchCleaningUser)
	http.HandleFunc("DELETE /admin/cleaning/point", s.deleteCleaningPoint)
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

func (s *Service) mandagsstad(w http.ResponseWriter, r *http.Request) {
	topUsers, err := getTop10CleanersWithPoints(s.db, s.ctx)
	date := pgtype.Date{
		Time:  SemesterDate(),
		Valid: true,
	}
	topUsersYear, err := getTop10CleanersWithPointsSince(s.db, s.ctx, date)

	var userStringList []string

	for _, user := range topUsers {
		userStringList = append(userStringList, user.Kthid)
	}

	var userStringListYear []string

	for _, user := range topUsersYear {
		userStringListYear = append(userStringListYear, user.Kthid)
	}

	usersAll, err := getSSOUsers(userStringList)
	if err != nil {
		slog.Error("Failed to get SSO users for mandagsstad", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	usersYear, err := getSSOUsers(userStringListYear)
	if err != nil {
		slog.Error("Failed to get SSO users for year", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := struct {
		UsersAll []struct {
			Name   string
			Points int
			ImgUrl string
		}
		UsersYear []struct {
			Name   string
			Points int
			ImgUrl string
		}
	}{
		UsersAll: make([]struct {
			Name   string
			Points int
			ImgUrl string
		}, len(topUsers)),
		UsersYear: make([]struct {
			Name   string
			Points int
			ImgUrl string
		}, len(topUsersYear)),
	}

	for i, user := range topUsers {
		for _, ssoUser := range usersAll {
			if ssoUser.KthId == user.Kthid {
				data.UsersAll[i].Name = fmt.Sprintf("%s %s", ssoUser.FirstName, ssoUser.FamilyName)
				data.UsersAll[i].Points = user.Points
				data.UsersAll[i].ImgUrl = ssoUser.Picture
				break
			}
		}
	}

	for i, user := range topUsersYear {
		for _, ssoUser := range usersYear {
			if ssoUser.KthId == user.Kthid {
				data.UsersYear[i].Name = fmt.Sprintf("%s %s", ssoUser.FirstName, ssoUser.FamilyName)
				data.UsersYear[i].Points = user.Points
				data.UsersYear[i].ImgUrl = ssoUser.Picture
				break
			}
		}
	}

	if err := s.t.ExecuteTemplate(w, "cleaning.html", data); err != nil {
		slog.Error("Failed to execute template", "error", err)
	}

}

type tvImg struct {
	Src    string
	Width  int
	Height int
}

type cleaningTVData struct {
	Users []struct {
		Name   string
		Points int
		ImgUrl string
	}
	Img1 tvImg
	Img2 tvImg
}

func buildCleaningTVData(topUsers []CleanerPoints, ssoUsers []SsoUser, img1, img2 tvImg) cleaningTVData {
	data := cleaningTVData{
		Users: make([]struct {
			Name   string
			Points int
			ImgUrl string
		}, len(topUsers)),
		Img1: img1,
		Img2: img2,
	}
	for i, u := range topUsers {
		for _, sso := range ssoUsers {
			if sso.KthId == u.Kthid {
				data.Users[i].Name = fmt.Sprintf("%s %s", sso.FirstName, sso.FamilyName)
				data.Users[i].Points = u.Points
				data.Users[i].ImgUrl = sso.Picture
				break
			}
		}
	}
	return data
}

func (s *Service) mandagsstadAllTime(w http.ResponseWriter, r *http.Request) {
	topUsers, err := getTop10CleanersWithPoints(s.db, s.ctx)
	if err != nil {
		slog.Error("Failed to get top cleaners", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	var kthids []string
	for _, u := range topUsers {
		kthids = append(kthids, u.Kthid)
	}

	ssoUsers, err := getSSOUsers(kthids)
	if err != nil {
		slog.Error("Failed to get SSO users", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := buildCleaningTVData(topUsers, ssoUsers,
		tvImg{"/static/Mandagsstad-pink.gif", 600, 125},
		tvImg{"/static/Mandagsstad-all-time.gif", 300, 75},
	)

	if err := s.t.ExecuteTemplate(w, "cleaningTV.html", data); err != nil {
		slog.Error("Failed to execute template", "error", err)
	}
}

func (s *Service) mandagsstadYear(w http.ResponseWriter, r *http.Request) {
	date := pgtype.Date{
		Time:  SemesterDate(),
		Valid: true,
	}
	topUsers, err := getTop10CleanersWithPointsSince(s.db, s.ctx, date)
	if err != nil {
		slog.Error("Failed to get top cleaners for year", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	var kthids []string
	for _, u := range topUsers {
		kthids = append(kthids, u.Kthid)
	}

	ssoUsers, err := getSSOUsers(kthids)
	if err != nil {
		slog.Error("Failed to get SSO users", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := buildCleaningTVData(topUsers, ssoUsers,
		tvImg{"/static/Mandagsstad-anim.gif", 360, 75},
		tvImg{"/static/i-ar.gif", 105, 56},
	)

	if err := s.t.ExecuteTemplate(w, "cleaningTV.html", data); err != nil {
		slog.Error("Failed to execute template", "error", err)
	}
}

func (s *Service) redirectToAbout(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/about", http.StatusSeeOther)
}
