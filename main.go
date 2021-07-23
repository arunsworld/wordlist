package main

import (
	"embed"
	"flag"
	"log"
	"net/http"

	"github.com/arunsworld/website/pkg/website"
)

//go:embed web/*
var webContent embed.FS

func main() {
	port := flag.Int("port", 6123, "port to start chat server on")
	db := flag.String("db", "db.db", "location of db")
	flag.Parse()

	ws := website.NewWebsite(*db, "mywebsite")
	defer ws.Close()

	ws.EnableStatic()
	if err := ws.EnableLogin(); err != nil {
		ws.Close()
		log.Fatal(err)
	}

	setupHome(ws)
	setupWordlist(ws)
	setupQuiz(ws)
	setupScores(ws)

	if err := ws.Serve(*port); err != nil {
		ws.Close()
		log.Fatal(err)
	}
}

func setupHome(ws *website.Website) {
	data, err := webContent.ReadFile("web/html/home.html")
	if err != nil {
		panic(err)
	}
	index := ws.Router().Path("/").Subrouter()
	// index.Use(ws.EnsureAuthMiddleware(website.AuthMiddlewareConfig{}))
	index.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write(data)
	})
}