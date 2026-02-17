package main

import (
	"log/slog"
	"net/http"
)

func (s *Service) sodaPage(w http.ResponseWriter, r *http.Request) {
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

	data := fridgeData{
		Sodas:  sodas,
		Snacks: snacks,
	}
	if err := s.t.ExecuteTemplate(w, "soda.html", data); err != nil {
		slog.Error("Failed to execute template:", err)
		http.Error(w, "Template render error", http.StatusInternalServerError)
	}
}
