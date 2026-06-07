package main

import (
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type adminData struct {
	Date             string
	SemesterDate     string
	User             string
	FridgePerm       bool
	FridgeData       fridgeData
	CleanPerm        bool
	CleanLeaderboard []CleanerPoints
}

func FridgePerms(w http.ResponseWriter, r *http.Request, s *Service) bool {
	_, perms, err := Auth(w, r, s.oauth2Config)
	if err != nil {
		slog.Error("Authentication error:", err)
		http.Error(w, "Authentication error", http.StatusInternalServerError)
		return false
	}
	if !(HasPermission(perms, "fridge") || HasPermission(perms, "admin")) {
		http.Redirect(w, r, "/about", http.StatusSeeOther)
		return false
	}
	return true
}

func CleanPerms(w http.ResponseWriter, r *http.Request, s *Service) bool {
	_, perms, err := Auth(w, r, s.oauth2Config)
	if err != nil {
		slog.Error("Authentication error:", err)
		http.Error(w, "Authentication error", http.StatusInternalServerError)
		return false
	}
	if !(HasPermission(perms, "clean") || HasPermission(perms, "admin")) {
		http.Redirect(w, r, "/about", http.StatusSeeOther)
		return false
	}
	return true
}

func SemesterDate() time.Time {
	now := time.Now()
	year := now.Year()
	if now.Month() >= time.January {
		year -= 1
	}
	month := time.Month(8)
	return time.Date(year, month, 1, 0, 0, 0, 0, now.Location())
}

func leaderboardData(s *Service) ([]CleanerPoints, error) {
	date := SemesterDate()
	pgDate := pgtype.Date{
		Time:  date,
		Valid: true,
	}
	return getAllCleanersSince(s.db, s.ctx, pgDate)
}

func (s *Service) adminPage(w http.ResponseWriter, r *http.Request) {
	user, perms, err := Auth(w, r, s.oauth2Config)
	if err != nil {
		slog.Error("Authentication error:", err)
		http.Error(w, "Authentication error", http.StatusInternalServerError)
		return
	}
	cleanPerm := HasPermission(perms, "clean")
	fridgePerm := HasPermission(perms, "fridge")
	adminPerm := HasPermission(perms, "admin")

	if !(adminPerm || fridgePerm || cleanPerm) {
		http.Redirect(w, r, "/about", http.StatusSeeOther)
		return
	}

	var fridge fridgeData
	if fridgePerm || adminPerm {
		sodas, err := getSodas(s.db, s.ctx)
		if err != nil {
			slog.Error("Failed to query soda fridge:", err)
			http.Error(w, "Cant query soda fridge", http.StatusInternalServerError)
			return
		}
		snacks, err := getSnacks(s.db, s.ctx)
		if err != nil {
			slog.Error("Failed to query snack fridge:", err)
			http.Error(w, "Cant query snack fridge", http.StatusInternalServerError)
			return
		}

		fridge = fridgeData{
			Sodas:  sodas,
			Snacks: snacks,
		}
	}

	cleanLeaderboard, err := leaderboardData(s)
	if err != nil {
		slog.Error("Failed to get clean leaderboard data:", err)
		http.Error(w, "Failed to get clean leaderboard data", http.StatusInternalServerError)
		return
	}

	data := adminData{
		Date:             time.Now().Format("2006-01-02"),
		SemesterDate:     SemesterDate().Format("2006-01-02"),
		User:             user,
		FridgePerm:       fridgePerm || adminPerm,
		FridgeData:       fridge,
		CleanPerm:        cleanPerm || adminPerm,
		CleanLeaderboard: cleanLeaderboard,
	}

	if err := s.t.ExecuteTemplate(w, "admin.html", data); err != nil {
		slog.Error("Failed to execute template:", err)
	}
}

func (s *Service) addFridgeItem(w http.ResponseWriter, r *http.Request) {
	if !FridgePerms(w, r, s) {
		return
	}

	if err := r.ParseForm(); err != nil {
		slog.Error("Failed to parse form:", err)
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	t := r.PathValue("t")
	if t != "soda" && t != "snack" {
		http.Error(w, "Invalid fridge type", http.StatusBadRequest)
		return
	}
	priceStr := r.FormValue("price")
	p, err := strconv.ParseFloat(priceStr, 32)
	price := float32(p)
	if err != nil {
		slog.Error("Failed to parse price:", err)
		http.Error(w, "Invalid price format", http.StatusBadRequest)
		return
	}

	if err := addFridgeItem(s.db, s.ctx, name, t, price); err != nil {
		slog.Error("Failed to add fridge item:", err)
		http.Error(w, "Failed to add fridge item", http.StatusInternalServerError)
		return
	}

	data := struct {
		Name  string
		Price float32
	}{
		name,
		price,
	}

	slog.Info("Add fridge item requested", "item", name, "type", t, "price", price)

	if err := s.t.ExecuteTemplate(w, "fridgeItemRow.html", data); err != nil {
		slog.Error("Failed to execute template:", err)
		return
	}

	items, err := s.sodaItems(w, r)
	if err != nil {
		return
	}
	s.sseEvents <- items
}

func (s *Service) editFridgeItem(w http.ResponseWriter, r *http.Request) {
	if !FridgePerms(w, r, s) {
		return
	}

	name := r.PathValue("n")
	item, err := getFridgeItem(s.db, s.ctx, name)
	if err != nil {
		slog.Error("Failed to get fridge item:", err)
		http.Error(w, "Failed to get fridge item", http.StatusBadRequest)
		return
	}

	data := struct {
		Name  string
		Price float32
		Ftype string
	}{
		Name:  item.Name,
		Price: item.Price,
		Ftype: item.Type,
	}

	if err := s.t.ExecuteTemplate(w, "editForm.html", data); err != nil {
		slog.Error("Failed to execute template:", err)
	}
}

func (s *Service) cancelFridgeEdit(w http.ResponseWriter, r *http.Request) {
	if !FridgePerms(w, r, s) {
		return
	}

	name := r.PathValue("n")
	item, err := getFridgeItem(s.db, s.ctx, name)
	if err != nil {
		slog.Error("Failed to get fridge item:", err)
		http.Error(w, "Failed to get fridge item", http.StatusBadRequest)
		return
	}

	data := struct {
		Name  string
		Price float32
		Ftype string
	}{
		Name:  item.Name,
		Price: item.Price,
		Ftype: item.Type,
	}

	if err := s.t.ExecuteTemplate(w, "cancelForm.html", data); err != nil {
		slog.Error("Failed to execute template:", err)
	}

}

func (s *Service) saveFridgeItemEdit(w http.ResponseWriter, r *http.Request) {
	if !FridgePerms(w, r, s) {
		return
	}

	if err := r.ParseForm(); err != nil {
		slog.Error("Failed to parse form:", err)
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	oldName := r.PathValue("n")
	name := r.FormValue("name")
	p := r.FormValue("price")
	price, err := strconv.ParseFloat(p, 32)
	if err != nil {
		slog.Error("Failed to parse price:", err)
		http.Error(w, "Invalid price format", http.StatusBadRequest)
		return
	}

	item, err := getFridgeItem(s.db, s.ctx, oldName)
	if err != nil {
		slog.Error("Failed to get fridge item:", err)
		http.Error(w, "Failed to get fridge item", http.StatusBadRequest)
		return
	}

	if err := updateFridgeItem(s.db, s.ctx, oldName, name, float32(price)); err != nil {
		slog.Error("Failed to update fridge item:", err)
		http.Error(w, "Failed to update fridge item", http.StatusInternalServerError)
		return
	}

	data := struct {
		Name  string
		Price float32
		Ftype string
	}{
		Name:  name,
		Price: float32(price),
		Ftype: item.Type,
	}

	if err := s.t.ExecuteTemplate(w, "saveItem.html", data); err != nil {
		slog.Error("Failed to execute template:", err)
	}
	items, err := s.sodaItems(w, r)
	if err != nil {
		return
	}
	s.sseEvents <- items
}

func (s *Service) confirmDeleteFridgeItem(w http.ResponseWriter, r *http.Request) {
	if !FridgePerms(w, r, s) {
		return
	}

	name := r.PathValue("n")

	data := struct {
		Name string
	}{
		Name: name,
	}

	if err := s.t.ExecuteTemplate(w, "confirmDelete.html", data); err != nil {
		slog.Error("Failed to execute template:", err)
	}

}

func (s *Service) removeFridgeItem(w http.ResponseWriter, r *http.Request) {
	if !FridgePerms(w, r, s) {
		return
	}

	name := r.PathValue("n")
	if err := deleteFridgeItem(s.db, s.ctx, name); err != nil {
		slog.Error("Failed to get fridge item:", err)
		http.Error(w, "Failed to get fridge item", http.StatusBadRequest)
		return
	}
	items, err := s.sodaItems(w, r)
	if err != nil {
		return
	}
	s.sseEvents <- items
}

type Item struct {
	Name     string `json:"name"`
	Priority int    `json:"priority"`
}

func (s *Service) updateFridgeItemPriority(w http.ResponseWriter, r *http.Request) {
	if !FridgePerms(w, r, s) {
		return
	}

	r.ParseForm()

	itemsJSON := r.FormValue("items")

	var items []Item
	err := json.Unmarshal([]byte(itemsJSON), &items)
	if err != nil {
		log.Println("JSON error:", err)
		http.Error(w, "bad request", 400)
		return
	}

	for _, item := range items {
		if err := updateFridgeItemPriority(s.db, s.ctx, item.Name, item.Priority); err != nil {
			slog.Error("Failed to update fridge item priority:", err)
			http.Error(w, "Failed to update fridge item priority", http.StatusInternalServerError)
			return
		}
	}

	newItems, err := s.sodaItems(w, r)
	if err != nil {
		return
	}
	s.sseEvents <- newItems
}

func (s *Service) searchSSOusers(w http.ResponseWriter, r *http.Request) {
	if !(FridgePerms(w, r, s) || CleanPerms(w, r, s)) {
		return
	}

	id := r.PathValue("id")
	name := r.URL.Query().Get("name")

	if name == "" {
		data := struct {
			Options []SsoUser
			Id      string
		}{
			Options: []SsoUser{},
			Id:      id,
		}
		if err := s.t.ExecuteTemplate(w, "ssoSearch.html", data); err != nil {
			slog.Error("Failed to execute template:", err)
			http.Error(w, "Failed to render search results", http.StatusInternalServerError)
		}
		return
	}

	client := &http.Client{}
	url := fmt.Sprintf("%s/api/search?query=%s", os.Getenv("SSO_URL"), name)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error("Failed to create request:", err)
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Failed to make request:", err)
		http.Error(w, "Failed to make request", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Error("Unexpected status code:", "status", resp.StatusCode)
		http.Error(w, "Unexpected status code from SSO", http.StatusInternalServerError)
		return
	}

	var users []SsoUser
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		slog.Error("Failed to decode response:", err)
		http.Error(w, "Failed to decode response from SSO", http.StatusInternalServerError)
		return
	}

	slog.Info("SSO search requested", "query", name, "results", users)

	data := struct {
		Options []SsoUser
		Id      string
	}{
		Options: users,
		Id:      id,
	}

	if err := s.t.ExecuteTemplate(w, "ssoSearch.html", data); err != nil {
		slog.Error("Failed to execute template:", err)
		http.Error(w, "Failed to render search results", http.StatusInternalServerError)
		return
	}
}

func (s *Service) addCleanPoint(w http.ResponseWriter, r *http.Request) {
	if !CleanPerms(w, r, s) {
		return
	}

	cleanLeaderboard, err := leaderboardData(s)

	if err := r.ParseForm(); err != nil {
		slog.Error("Failed to parse form:", err)
		data := struct {
			Color   string
			Message string
			Users   []CleanerPoints
		}{
			Color:   "pico-color-red-500",
			Message: "❌ Något gick fel med att ge poäng",
			Users:   cleanLeaderboard,
		}
		if err := s.t.ExecuteTemplate(w, "adminCleanLeaderboard.html", data); err != nil {
			slog.Error("Failed to execute template:", err)
		}

		return
	}

	kthid := r.FormValue("name")
	dateStr := r.FormValue("date")
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		slog.Error("Failed to parse date:", err)
		data := struct {
			Color   string
			Message string
			Users   []CleanerPoints
		}{
			Color:   "pico-color-red-500",
			Message: "❌ Ogiltigt datumformat",
			Users:   cleanLeaderboard,
		}
		if err := s.t.ExecuteTemplate(w, "adminCleanLeaderboard.html", data); err != nil {
			slog.Error("Failed to execute template:", err)
		}

		return
	}
	pgDate := pgtype.Date{
		Time:  date,
		Valid: true,
	}

	if err := addCleanPoint(s.db, s.ctx, kthid, pgDate); err != nil {
		slog.Error("Failed to add clean point:", err)
		data := struct {
			Color   string
			Message string
			Users   []CleanerPoints
		}{
			Color:   "pico-color-red-500",
			Message: "❌ Något gick fel med att ge poäng",
			Users:   cleanLeaderboard,
		}
		if err := s.t.ExecuteTemplate(w, "adminCleanLeaderboard.html", data); err != nil {
			slog.Error("Failed to execute template:", err)
		}
		return
	}

	slog.Info("Added clean point", "kthid", kthid, "date", dateStr)

	cleanLeaderboard, err = leaderboardData(s)

	data := struct {
		Color   string
		Message string
		Users   []CleanerPoints
	}{
		Color:   "pico-color-green-400",
		Message: "✅ Städ poäng till " + kthid,
		Users:   cleanLeaderboard,
	}
	if err := s.t.ExecuteTemplate(w, "adminCleanLeaderboard.html", data); err != nil {
		slog.Error("Failed to execute template:", err)
	}
}

func (s *Service) searchCleanUser(w http.ResponseWriter, r *http.Request) {
	if !CleanPerms(w, r, s) {
		return
	}

	if err := r.ParseForm(); err != nil {
		slog.Error("Failed to parse form:", err)
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	kthid := r.FormValue("name")
	from := r.FormValue("from")
	to := r.FormValue("to")

	fromDate, err := time.Parse("2006-01-02", from)
	if err != nil {
		slog.Error("Failed to parse from date:", err)
		http.Error(w, "Invalid from date format", http.StatusBadRequest)
		return
	}

	toDate, err := time.Parse("2006-01-02", to)
	if err != nil {
		slog.Error("Failed to parse to date:", err)
		http.Error(w, "Invalid to date format", http.StatusBadRequest)
		return
	}

	fromPgDate := pgtype.Date{
		Time:  fromDate,
		Valid: true,
	}
	toPgDate := pgtype.Date{
		Time:  toDate,
		Valid: true,
	}

	slog.Info("Search clean user requested", "kthid", kthid, "from", from, "to", to)

	cleanPoints, err := getCleanPointsByKthid(s.db, s.ctx, kthid, fromPgDate, toPgDate)
	if err != nil {
		slog.Error("Failed to get clean points for user:", err)
		http.Error(w, "Failed to get clean points for user", http.StatusInternalServerError)
		return
	}

	data := struct {
		Dates []string
		Kthid string
	}{
		Kthid: kthid,
		Dates: []string{},
	}

	for _, cp := range cleanPoints {
		data.Dates = append(data.Dates, cp.Time.Format("2006-01-02"))
	}

	slog.Info("Found clean points for user", "kthid", kthid, "points", data.Dates)

	if err := s.t.ExecuteTemplate(w, "cleanUserList.html", data); err != nil {
		slog.Error("Failed to execute template:", err)
	}
}

func (s *Service) deleteCleanPoint(w http.ResponseWriter, r *http.Request) {
	if !CleanPerms(w, r, s) {
		return
	}

	kthid := r.URL.Query().Get("name")
	dateStr := r.URL.Query().Get("date")
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		slog.Error("Failed to parse date:", err)
		http.Error(w, "Invalid date format", http.StatusBadRequest)
		return
	}
	pgDate := pgtype.Date{
		Time:  date,
		Valid: true,
	}

	if err := removeCleanPoint(s.db, s.ctx, kthid, pgDate); err != nil {
		slog.Error("Failed to remove clean point:", err)
		http.Error(w, "Failed to remove clean point", http.StatusInternalServerError)
		return
	}

	slog.Info("Removed clean point", "kthid", kthid, "date", dateStr)
}
