package main

import (
	"log/slog"
	"net/http"
	"strconv"
)

type adminData struct {
	User       string
	FridgePerm bool
	FridgeData fridgeData
	CleanPerm  bool
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

	data := adminData{
		User:       user,
		FridgePerm: fridgePerm || adminPerm,
		FridgeData: fridge,
		CleanPerm:  cleanPerm || adminPerm,
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
	s.sseEvents<-items
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
	s.sseEvents<-items
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
	s.sseEvents<-items
}
