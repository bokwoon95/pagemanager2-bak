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

const home = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <link rel="stylesheet" href="/pm-plugins/pagemanager/tachyons.css">
  <link rel="stylesheet" href="/pm-plugins/pagemanager/style.css">
  <title>Home</title>
</head>
<body class="bg-light-gray sans-serif">
  <div class="pa4">
    <h1 class="f4-ns f1-m f-subheadline-l word-wrap">Hello World</h1>
	<p><a href="/pm-dashboard">Dashboard</a></p>
	<p><a href="/pm-superadmin-login">Superadmin Log In</a></p>
  </div>
</body>
</html>`

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
		w.Write([]byte(home))
	})
	fmt.Println("listening on :80")
	http.ListenAndServe(":80", mux)
}
