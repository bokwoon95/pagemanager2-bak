package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/bokwoon95/erro"
	"github.com/bokwoon95/pagemanager"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
)

func main() {
	flag.Parse()
	pm, err := pagemanager.New()
	if err != nil {
		log.Fatalln(erro.Wrap(err))
	}
	mux := chi.NewRouter()
	mux.Use(middleware.Compress(5))
	mux.Use(pm.PageManager)
	mux.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<h1>hello world</h1><br><a href="/pm-superadmin-login">Superadmin Log In</a>`))
		w.Write([]byte(`<br><a href="/pm-dashboard">Dashboard</a>`))
	})
	fmt.Println("listening on :80")
	http.ListenAndServe(":80", mux)
}
