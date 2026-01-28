package handlers

import (
	"encoding/json"
	"net/http"
	"tempmail/internal/database"
	"tempmail/internal/models"
	"tempmail/internal/services"
	"time"
)

// HandleStatus verifica o estado atual do sistema
func HandleStatus(w http.ResponseWriter, r *http.Request) {
	setupDone := database.IsSetupDone()
	cfg, _ := database.GetConfig()
	configDone := cfg.CFToken != ""

	json.NewEncoder(w).Encode(map[string]interface{}{
		"setup_done":  setupDone,
		"config_done": configDone,
	})
}

// HandleSetup cria o primeiro usuário administrador
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

// HandleLogin autentica o usuário e retorna o JWT
func HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Dados inválidos", http.StatusBadRequest)
		return
	}

	var hashedPassword string
	err := database.DB.QueryRow("SELECT password FROM users WHERE username = ?", req.Username).Scan(&hashedPassword)

	if err != nil || !services.CheckPasswordHash(req.Password, hashedPassword) {
		http.Error(w, "Usuário ou senha inválidos", http.StatusUnauthorized)
		return
	}

	token, _ := services.GenerateToken(req.Username)
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

// HandleLogout apenas confirma o encerramento da sessão
func HandleLogout(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Logout realizado com sucesso"})
}

// HandleChangePassword altera a senha do usuário logado
func HandleChangePassword(w http.ResponseWriter, r *http.Request) {
	// Obtém o usuário do contexto (definido pelo AuthMiddleware)
	username := r.Context().Value("username").(string)

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Dados inválidos", http.StatusBadRequest)
		return
	}

	// Verifica a senha atual
	var hashedPassword string
	err := database.DB.QueryRow("SELECT password FROM users WHERE username = ?", username).Scan(&hashedPassword)
	if err != nil {
		http.Error(w, "Usuário não encontrado", http.StatusNotFound)
		return
	}

	if !services.CheckPasswordHash(req.CurrentPassword, hashedPassword) {
		http.Error(w, "Senha atual incorreta", http.StatusUnauthorized)
		return
	}

	// Criptografa e atualiza a nova senha
	newHashedPassword, _ := services.HashPassword(req.NewPassword)
	_, err = database.DB.Exec("UPDATE users SET password = ? WHERE username = ?", newHashedPassword, username)
	if err != nil {
		http.Error(w, "Erro ao atualizar senha", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Senha atualizada com sucesso"})
}

// HandleTestCloudflare valida se as credenciais funcionam
func HandleTestCloudflare(w http.ResponseWriter, r *http.Request) {
	var cfg models.Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, "Dados inválidos", http.StatusBadRequest)
		return
	}

	_, err := services.CfGetAccountID(cfg)
	if err != nil {
		http.Error(w, "Falha na conexão: "+err.Error(), http.StatusUnauthorized)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Conexão OK!"})
}