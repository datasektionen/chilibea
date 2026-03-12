package main

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
)

func (s *Service) sodaPage(w http.ResponseWriter, r *http.Request) {
	data, err := s.sodaItems(w, r)
	if err != nil {
		return
	}

	if err := s.t.ExecuteTemplate(w, "soda.html", data); err != nil {
		slog.Error("Failed to execute template:", err)
		http.Error(w, "Template render error", http.StatusInternalServerError)
	}
}


func (s *Service) sodaItems(w http.ResponseWriter, r *http.Request) (data fridgeData, err error) {
	sodas, err := getSodas(s.db, s.ctx)
	if err != nil {
		slog.Error("Failed to query soda fridge:", err)
		http.Error(w, "Cant query soda fridge", http.StatusInternalServerError)
		return data, err
	}
	snacks, err := getSnacks(s.db, s.ctx)
	if err != nil {
		slog.Error("Failed to query snack fridge:", err)
		http.Error(w, "Cant query snack fridge", http.StatusInternalServerError)
		return data, err
	}

	data = fridgeData{
		Sodas:  sodas,
		Snacks: snacks,
	}

	return data, nil
}

func (s *Service) sseFridge(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		slog.Error("Streaming unsupported by the response writer")
		return
	}

	slog.Info("Client connected to SSE stream")

	for {
		select {
		case <-r.Context().Done():
			slog.Info("Client disconnected from SSE stream")
			return
		case data := <-s.sseEvents:
			slog.Info("Updating fridge")
			var buf bytes.Buffer
			if err := s.t.ExecuteTemplate(&buf, "sodaList", data); err != nil {
				slog.Error("Failed to execute template:", err)
				http.Error(w, "Template render error", http.StatusInternalServerError)
				return
			}
			html := strings.ReplaceAll(buf.String(), "\n", "")
			fmt.Fprintf(w, "event: fridge\ndata: %s\n\n", html)

			flusher.Flush()
		}
	}
}
