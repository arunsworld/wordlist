package main

import (
	"embed"
	"encoding/csv"
	"log"
	"os"

	"github.com/arunsworld/wordlist"
	"github.com/arunsworld/wordlist/pkg/website"
)

func main() {
	f, err := os.Open("words.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	cr := csv.NewReader(f)
	cr.Comma = '\t'
	records, err := cr.ReadAll()
	if err != nil {
		log.Fatal(err)
	}

	if err := saveRecords(records); err != nil {
		log.Fatal(err)
	}
}

func saveRecords(records [][]string) error {
	ws := website.NewWebsite("db.db", "wordlist", embed.FS{})
	defer ws.Close()

	db := ws.DB()

	for _, record := range records {
		w := wordlist.Word{Word: record[0], Meaning: record[1]}
		if err := w.Create(db); err != nil {
			log.Printf("error savinv word: %s: %v", w.Word, err)
		}
	}
	return nil
}
