package main

import (
	"log"
	"net/http"
	"os"

	autodemo "github.com/koeng101/autodemo/src"
)

func main() {
	app := autodemo.InitializeApp("test.db")

	// Serve application
	s := &http.Server{
		Addr:    ":" + os.Getenv("PORT"),
		Handler: app.Router,
	}
	log.Fatal(s.ListenAndServe())
}
