package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
)

func main() {
	// Create root context
	rootCtx := context.Background()
	ctx, cancel := context.WithCancel(rootCtx)
	defer cancel()

	setupManager(ctx)

	log.Println("Starting server")
	err := http.ListenAndServeTLS(":8080", "server.crt", "server.key", nil)
	if err != nil {
		log.Fatal(err)
	}
}

func setupManager(ctx context.Context) {
	log.Println("Setting up manager")

	manager := NewManager(ctx)

	http.Handle("/", http.FileServer(http.Dir("./public")))
	http.HandleFunc("/login", manager.loginHandler)
	http.HandleFunc("/ws", manager.wsHandler)

	http.HandleFunc("/debug", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, len(manager.clients))
	})
}
