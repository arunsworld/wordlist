package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/arunsworld/website/pkg/website"
)

func setupScores(ws *website.Website) {
	scoresHTML, err := webContent.ReadFile("web/html/scores.html")
	if err != nil {
		panic(err)
	}

	scores := ws.Router().Path("/scores/").Methods("GET").Subrouter()
	scores.Use(ws.EnsureAuthMiddleware(website.AuthMiddlewareConfig{}))
	scores.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write(scoresHTML)
	})

	scoresAPI := ws.Router().PathPrefix("/scores-api/").Methods("GET").Subrouter()
	scoresAPI.Use(ws.EnsureAuthMiddleware(website.AuthMiddlewareConfig{}))
	scoresAPI.HandleFunc("/scores/", func(w http.ResponseWriter, r *http.Request) {
		user := ws.AuthenticatedUser(r)
		completedQuizes := []CompletedQuiz{}
		result := ws.DB().Where("user_id = ?", user.ID).Find(&completedQuizes)
		if result.Error != nil {
			log.Printf("error reading completed quizes from DB: %v", err)
			http.Error(w, "unable to read scores", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/javascript")
		if err := json.NewEncoder(w).Encode(completedQuizes); err != nil {
			log.Printf("error encoding completed quizes: %v", err)
			http.Error(w, "unable to read scores", http.StatusInternalServerError)
			return
		}
	})
}
