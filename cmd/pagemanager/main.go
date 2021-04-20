package main

import (
	_ "embed"
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/bokwoon95/pagemanager"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
)

//go:embed main.html
var mainfile string

func main() {
	flag.Parse()
	pm, err := pagemanager.New()
	if err != nil {
		log.Fatalln(err)
	}
	mux := chi.NewRouter()
	mux.Use(middleware.Compress(5))
	mux.Use(pm.PageManager)
	mux.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(mainfile))
	})
	fmt.Println("listening on :80")
	http.ListenAndServe(":80", mux)
}
