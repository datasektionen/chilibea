package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"
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

	for {
		select {
		case <-r.Context().Done():
			// Client disconnected
			return
		default:
			slog.Info("Generating new SSE message for fridge updates")
			// Simulate generating an updated list (usually from a DB or channel)
			htmlSnippet := "<li>Item " + time.Now().Format("15:04:05") + "</li>"

			// 4. Format the SSE message
			// SSE requires the "data: " prefix and two newlines at the end
			fmt.Fprintf(w, "data: %s\n\n", htmlSnippet)

			// 5. Push data to the client immediately
			flusher.Flush()

			// Wait before next update
			time.Sleep(2 * time.Second)
		}
	}
}
