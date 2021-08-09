package main

import (
	"embed"
	"flag"
	"log"
	"net/http"

	"github.com/arunsworld/wordlist"
	"github.com/arunsworld/wordlist/pkg/website"
)

//go:embed web/*
var webContent embed.FS

func main() {
	port := flag.Int("port", 6123, "port to start app server on")
	db := flag.String("db", "db.db", "location of db")
	flag.Parse()

	ws := website.NewWebsite(*db, "wordlist", webContent)
	defer ws.Close()

	ws.EnableStatic()
	if err := ws.EnableLogin(); err != nil {
		ws.Close()
		log.Fatal(err)
	}

	setupHome(ws)
	setupPingForKeepAlive(ws)
	wordlist.SetupWordlist(ws)
	wordlist.SetupQuiz(ws)
	wordlist.SetupScores(ws)

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
	index.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write(data)
	})
}

func setupPingForKeepAlive(ws *website.Website) {
	ping := ws.Router().Path("/ping/").Subrouter()
	ping.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Write([]byte("{}"))
	})
}
