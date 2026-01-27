package main

import (
	"fmt"
	"log"
	"net/http"
	"tempmail/internal/config"
	"tempmail/internal/database"
	"tempmail/internal/handlers"

	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()
	port := config.GetPort()
	database.InitDB()

	http.Handle("/", http.FileServer(http.Dir("./static")))

	// Rotas Públicas (Auth e Status)
	http.HandleFunc("/api/status", handlers.HandleStatus)
	http.HandleFunc("/api/setup", handlers.HandleSetup)
	http.HandleFunc("/api/login", handlers.HandleLogin)

	// Rotas Protegidas (Exigem JWT)
	http.HandleFunc("/api/config", handlers.AuthMiddleware(handlers.HandleConfig))
	http.HandleFunc("/api/destinations", handlers.AuthMiddleware(handlers.HandleDestinations))
	http.HandleFunc("/api/check", handlers.AuthMiddleware(handlers.HandleCheck))
	http.HandleFunc("/api/create", handlers.AuthMiddleware(handlers.HandleCreate))
	http.HandleFunc("/api/pin", handlers.AuthMiddleware(handlers.HandlePin))
	http.HandleFunc("/api/active", handlers.AuthMiddleware(handlers.HandleListActive))
	http.HandleFunc("/api/history", handlers.AuthMiddleware(handlers.HandleHistory))
	http.HandleFunc("/api/delete", handlers.AuthMiddleware(handlers.HandleDelete))
	http.HandleFunc("/api/tags", handlers.AuthMiddleware(handlers.HandleTags))

	addr := ":" + port
	fmt.Printf("🚀 Sistema Mail com JWT rodando em http://localhost%s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}