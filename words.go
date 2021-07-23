package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/arunsworld/website/pkg/website"
	"gorm.io/gorm"
)

type Word struct {
	ID      uint   `gorm:"primaryKey"`
	Word    string `gorm:"unique"`
	Meaning string
}

func (w *Word) exists(db *gorm.DB) bool {
	result := db.Where("word = ?", w.Word).First(&Word{})
	return result.RowsAffected > 0
}

func (w *Word) create(db *gorm.DB) error {
	result := db.Create(w)
	if result.Error != nil {
		return result.Error
	}
	return nil
}

func setupWordlist(ws *website.Website) {
	if err := ws.DB().AutoMigrate(&Word{}); err != nil {
		panic(err)
	}

	wordlistHTML, err := webContent.ReadFile("web/html/words.html")
	if err != nil {
		panic(err)
	}

	authorizedUsers := []string{"admin", "abarua", "nomi"}

	wordlist := ws.Router().Path("/wordlist/").Methods("GET").Subrouter()
	wordlist.Use(ws.EnsureAuthMiddleware(website.AuthMiddlewareConfig{UserNames: authorizedUsers}))
	wordlist.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write(wordlistHTML)
	})

	wordlistOpenAPI := ws.Router().PathPrefix("/wordlist-open-api/").Methods("POST").Subrouter()
	wordlistOpenAPI.HandleFunc("/add/", func(w http.ResponseWriter, r *http.Request) {
		word := r.FormValue("word")
		word = strings.ToLower(strings.TrimSpace(word))
		meaning := r.FormValue("meaning")
		meaning = strings.ToLower(strings.TrimSpace(meaning))
		if word == "" || meaning == "" {
			log.Println("WARNING: Attempted word creation without word or meaning")
			http.Error(w, "Cannot create word without details", http.StatusBadRequest)
			return
		}

		wrd := &Word{
			Word:    word,
			Meaning: meaning,
		}
		if wrd.exists(ws.DB()) {
			http.Error(w, "word already exists", http.StatusBadRequest)
			return
		}
		if err := wrd.create(ws.DB()); err != nil {
			log.Printf("error creating word: %v", err)
			http.Error(w, "Could not create word", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/javascript")
		w.Write([]byte("{}"))
	})

	wordlistAPI := ws.Router().PathPrefix("/wordlist-api/").Methods("GET").Subrouter()
	wordlistAPI.Use(ws.EnsureAuthMiddleware(website.AuthMiddlewareConfig{UserNames: authorizedUsers, IsForAPI: true}))

	wordlistAPI.HandleFunc("/words/", func(w http.ResponseWriter, r *http.Request) {
		allWords := []Word{}
		result := ws.DB().Find(&allWords)
		if result.Error != nil {
			log.Printf("error reading words from DB: %v", err)
			http.Error(w, "unable to read words", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/javascript")
		if err := json.NewEncoder(w).Encode(allWords); err != nil {
			log.Printf("error marshaling data as JSON: %v", err)
			http.Error(w, "unable to read words", http.StatusInternalServerError)
			return
		}
	})

	wordlistPOSTAPI := ws.Router().PathPrefix("/wordlist-api/").Methods("POST").Subrouter()
	wordlistPOSTAPI.Use(ws.EnsureAuthMiddleware(website.AuthMiddlewareConfig{UserNames: authorizedUsers, IsForAPI: true}))

	wordlistPOSTAPI.HandleFunc("/save/", func(w http.ResponseWriter, r *http.Request) {
		wid_str := r.FormValue("wid")
		wid, err := strconv.Atoi(wid_str)
		if err != nil {
			log.Printf("error convering word ID to integer: %v", err)
			http.Error(w, "unable to save", http.StatusInternalServerError)
			return
		}
		if wid < 1 {
			log.Printf("received invalid word ID: %v", err)
			http.Error(w, "unable to save", http.StatusInternalServerError)
			return
		}
		meaning := r.FormValue("meaning")
		meaning = strings.ToLower(strings.TrimSpace(meaning))

		word := &Word{
			ID: uint(wid),
		}
		result := ws.DB().Model(word).Update("meaning", meaning)
		if result.Error != nil {
			log.Printf("error saving word update: %v", result.Error)
			http.Error(w, "unable to save", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/javascript")
		w.Write([]byte("{}"))
	})
}
