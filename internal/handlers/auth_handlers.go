package handlers

import (
	"encoding/json"
	"net/http"
	"tempmail/internal/database"
	"tempmail/internal/models"
	"tempmail/internal/services"
	"time"
)

func HandleStatus(w http.ResponseWriter, r *http.Request) {
	setupDone := database.IsSetupDone()
	json.NewEncoder(w).Encode(map[string]bool{"setup_done": setupDone})
}

func HandleSetup(w http.ResponseWriter, r *http.Request) {
	if database.IsSetupDone() {
		http.Error(w, "Setup já realizado", http.StatusForbidden)
		return
	}

	var req models.SetupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Dados inválidos", http.StatusBadRequest)
		return
	}

	hashedPassword, _ := services.HashPassword(req.Password)

	_, err := database.DB.Exec(
		"INSERT INTO users (username, password, full_name, created_at) VALUES (?, ?, ?, ?)",
		req.Username, hashedPassword, req.FullName, time.Now(),
	)

	if err != nil {
		http.Error(w, "Erro ao criar usuário", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	json.NewDecoder(r.Body).Decode(&req)

	var hashedPassword string
	err := database.DB.QueryRow("SELECT password FROM users WHERE username = ?", req.Username).Scan(&hashedPassword)

	if err != nil || !services.CheckPasswordHash(req.Password, hashedPassword) {
		http.Error(w, "Usuário ou senha inválidos", http.StatusUnauthorized)
		return
	}

	token, _ := services.GenerateToken(req.Username)
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}