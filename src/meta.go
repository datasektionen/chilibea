package main

import (
	"log/slog"
	"net/http"
)

func (s *Service) metaPage(w http.ResponseWriter, r *http.Request) {
	if err := s.t.ExecuteTemplate(w, "meta.html", nil); err != nil {
		slog.Error("Failed to execute template:", err)
	}
}
