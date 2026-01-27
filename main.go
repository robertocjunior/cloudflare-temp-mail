package main

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	_ "github.com/glebarez/go-sqlite"
	"github.com/joho/godotenv"
)

// --- Estruturas ---

type Config struct {
	CFToken string `json:"cf_token"`
	ZoneID  string `json:"zone_id"`
	Domain  string `json:"domain"`
}

type Destination struct {
	Tag      string `json:"tag"` // O ID do endereço no Cloudflare chama-se 'tag'
	Email    string `json:"email"`
	Verified string `json:"verified,omitempty"` // Data de verificação
}

type EmailEntry struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	Destination string    `json:"destination"`
	CreatedAt   time.Time `json:"created_at"`
	Active      bool      `json:"active"`
}

type CreateRequest struct {
	Destination string `json:"destination"`
}

// --- Listas de Nomes ---
var adjetivos = []string{"cansado", "calvo", "radioativo", "humilde", "furioso", "suspeito", "duvidoso", "crocante", "quase-rico", "lendario", "misterioso", "caotico", "triste", "iludido", "blindado", "agiota", "nutella", "raiz", "toxico", "quase-senior"}
var substantivos = []string{"boleto", "estagiario", "capivara", "gambiarra", "tijolo", "hacker", "pastel", "uno-com-escada", "coach", "cafe", "servidor", "bug", "golpe", "primo", "vaxco", "lider-tecnico", "git-blame", "deploy", "backup", "junior"}

// --- Globais ---
var db *sql.DB
var activeTimers = make(map[string]*time.Timer)
var timerMu sync.Mutex

func main() {
	godotenv.Load()
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	initDB()

	http.Handle("/", http.FileServer(http.Dir("./static")))

	// API
	http.HandleFunc("/api/config", handleConfig)
	http.HandleFunc("/api/destinations", handleDestinations)
	http.HandleFunc("/api/create", handleCreate)
	http.HandleFunc("/api/active", handleListActive)
	http.HandleFunc("/api/history", handleHistory)
	http.HandleFunc("/api/delete", handleDelete)

	addr := ":" + port
	fmt.Printf("🚀 Sistema Cloudflare Mail v4 rodando em http://localhost%s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

// --- Banco de Dados ---

func initDB() {
	var err error
	db, err = sql.Open("sqlite", "data.db")
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS config (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			cf_token TEXT,
			zone_id TEXT,
			domain TEXT
		);
		CREATE TABLE IF NOT EXISTS emails (
			id TEXT PRIMARY KEY,
			email TEXT UNIQUE,
			destination TEXT,
			created_at DATETIME,
			active BOOLEAN
		);
	`)
	if err != nil {
		log.Fatal("Erro ao migrar DB:", err)
	}
}

// --- Handlers ---

func handleConfig(w http.ResponseWriter, r *http.Request) {
	var currentCfg Config
	row := db.QueryRow("SELECT cf_token, zone_id, domain FROM config WHERE id = 1")
	row.Scan(&currentCfg.CFToken, &currentCfg.ZoneID, &currentCfg.Domain)

	if r.Method == http.MethodPost {
		var newCfg Config
		if err := json.NewDecoder(r.Body).Decode(&newCfg); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}

		finalToken := newCfg.CFToken
		if len(currentCfg.CFToken) > 0 && newCfg.CFToken == strings.Repeat("*", len(currentCfg.CFToken)) {
			finalToken = currentCfg.CFToken
		}

		_, err := db.Exec(`
			INSERT INTO config (id, cf_token, zone_id, domain) 
			VALUES (1, ?, ?, ?) 
			ON CONFLICT(id) DO UPDATE SET cf_token=excluded.cf_token, zone_id=excluded.zone_id, domain=excluded.domain
		`, finalToken, newCfg.ZoneID, newCfg.Domain)

		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	maskedCfg := currentCfg
	if len(currentCfg.CFToken) > 0 {
		maskedCfg.CFToken = strings.Repeat("*", len(currentCfg.CFToken))
	} else {
		maskedCfg.CFToken = ""
	}
	json.NewEncoder(w).Encode(maskedCfg)
}

// Handler Principal de Destinos (GET, POST, DELETE)
func handleDestinations(w http.ResponseWriter, r *http.Request) {
	cfg, err := getConfig()
	if err != nil {
		http.Error(w, "Configure o sistema primeiro", 400)
		return
	}

	// Precisamos do Account ID para qualquer operação aqui
	accountID, err := cfGetAccountID(cfg)
	if err != nil {
		http.Error(w, "Erro Account ID: "+err.Error(), 500)
		return
	}

	// LISTAR (GET)
	if r.Method == http.MethodGet {
		dests, err := cfGetVerifiedDestinations(cfg, accountID)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(dests)
		return
	}

	// ADICIONAR (POST)
	if r.Method == http.MethodPost {
		var req struct { Email string `json:"email"` }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", 400)
			return
		}
		if err := cfCreateDestination(cfg, accountID, req.Email); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	// DELETAR (DELETE)
	if r.Method == http.MethodDelete {
		destID := r.URL.Query().Get("id") // O ID aqui é a 'tag' do cloudflare
		if destID == "" {
			http.Error(w, "ID obrigatório", 400)
			return
		}
		if err := cfDeleteDestination(cfg, accountID, destID); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.WriteHeader(http.StatusOK)
		return
	}
	
	http.Error(w, "Method not allowed", 405)
}

func handleCreate(w http.ResponseWriter, r *http.Request) {
	cfg, err := getConfig()
	if err != nil {
		http.Error(w, "Configure o sistema primeiro!", 400)
		return
	}

	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Destination == "" {
		http.Error(w, "Destino obrigatório", 400)
		return
	}

	var alias string
	for i := 0; i < 10; i++ {
		candidato := fmt.Sprintf("%s@%s", gerarNomeEngracado(), cfg.Domain)
		if !emailExists(candidato) {
			alias = candidato
			break
		}
	}
	if alias == "" {
		http.Error(w, "Falha ao gerar nome único", 500)
		return
	}

	ruleID, err := cfCreateRule(cfg, alias, req.Destination)
	if err != nil {
		http.Error(w, "Erro Cloudflare: "+err.Error(), 500)
		return
	}

	_, err = db.Exec("INSERT INTO emails (id, email, destination, created_at, active) VALUES (?, ?, ?, ?, ?)",
		ruleID, alias, req.Destination, time.Now(), true)
	if err != nil {
		log.Println("Erro DB:", err)
	}

	startTimer(ruleID, alias, cfg)
	json.NewEncoder(w).Encode(map[string]string{"id": ruleID, "email": alias})
}

func handleListActive(w http.ResponseWriter, r *http.Request) {
	rows, _ := db.Query("SELECT id, email, destination, created_at, active FROM emails WHERE active = 1 ORDER BY created_at DESC")
	if rows != nil {
		defer rows.Close()
	}
	sendRows(w, rows)
}

func handleHistory(w http.ResponseWriter, r *http.Request) {
	rows, _ := db.Query("SELECT id, email, destination, created_at, active FROM emails ORDER BY created_at DESC")
	if rows != nil {
		defer rows.Close()
	}
	sendRows(w, rows)
}

func handleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	cfg, err := getConfig()
	if err != nil {
		http.Error(w, "Erro config", 500)
		return
	}

	cfDeleteRule(cfg, id)
	db.Exec("UPDATE emails SET active = 0 WHERE id = ?", id)

	timerMu.Lock()
	if t, ok := activeTimers[id]; ok {
		t.Stop()
		delete(activeTimers, id)
	}
	timerMu.Unlock()
	w.WriteHeader(http.StatusOK)
}

// --- Helpers ---

func getConfig() (Config, error) {
	var c Config
	err := db.QueryRow("SELECT cf_token, zone_id, domain FROM config WHERE id = 1").Scan(&c.CFToken, &c.ZoneID, &c.Domain)
	if c.CFToken == "" {
		return c, fmt.Errorf("no config")
	}
	return c, err
}

func emailExists(email string) bool {
	var exists bool
	db.QueryRow("SELECT EXISTS(SELECT 1 FROM emails WHERE email = ?)", email).Scan(&exists)
	return exists
}

func sendRows(w http.ResponseWriter, rows *sql.Rows) {
	var list []EmailEntry
	if rows != nil {
		for rows.Next() {
			var e EmailEntry
			rows.Scan(&e.ID, &e.Email, &e.Destination, &e.CreatedAt, &e.Active)
			list = append(list, e)
		}
	}
	if list == nil {
		list = []EmailEntry{}
	}
	json.NewEncoder(w).Encode(list)
}

func startTimer(id, email string, cfg Config) {
	timerMu.Lock()
	activeTimers[id] = time.AfterFunc(5*time.Minute, func() {
		log.Printf("⏰ Expirou: %s", email)
		cfDeleteRule(cfg, id)
		db.Exec("UPDATE emails SET active = 0 WHERE id = ?", id)
		timerMu.Lock()
		delete(activeTimers, id)
		timerMu.Unlock()
	})
	timerMu.Unlock()
}

func gerarNomeEngracado() string {
	nAdj, _ := rand.Int(rand.Reader, big.NewInt(int64(len(adjetivos))))
	nSub, _ := rand.Int(rand.Reader, big.NewInt(int64(len(substantivos))))
	nNum, _ := rand.Int(rand.Reader, big.NewInt(1000))
	return fmt.Sprintf("%s-%s-%d", substantivos[nSub.Int64()], adjetivos[nAdj.Int64()], nNum.Int64())
}

// --- Cloudflare API Calls ---

// 1. Criar Regra (Email Temporário)
func cfCreateRule(cfg Config, email, destination string) (string, error) {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/email/routing/rules", cfg.ZoneID)
	payload := map[string]interface{}{
		"enabled": true, "name": "Temp: " + email,
		"matchers": []interface{}{map[string]string{"type": "literal", "field": "to", "value": email}},
		"actions":  []interface{}{map[string]interface{}{"type": "forward", "value": []string{destination}}},
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+cfg.CFToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil { return "", err }
	defer resp.Body.Close()

	var res struct {
		Success bool `json:"success"`
		Result  struct{ ID string `json:"id"` } `json:"result"`
		Errors  []struct{ Message string `json:"message"` } `json:"errors"`
	}
	json.NewDecoder(resp.Body).Decode(&res)
	if !res.Success {
		if len(res.Errors) > 0 { return "", fmt.Errorf("%s", res.Errors[0].Message) }
		return "", fmt.Errorf("erro CF create rule")
	}
	return res.Result.ID, nil
}

// 2. Deletar Regra
func cfDeleteRule(cfg Config, id string) {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/email/routing/rules/%s", cfg.ZoneID, id)
	req, _ := http.NewRequest("DELETE", url, nil)
	req.Header.Set("Authorization", "Bearer "+cfg.CFToken)
	req.Header.Set("Content-Type", "application/json")
	http.DefaultClient.Do(req)
}

// 3. Pegar Account ID
func cfGetAccountID(cfg Config) (string, error) {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s", cfg.ZoneID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+cfg.CFToken)
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil { return "", err }
	defer resp.Body.Close()

	var res struct {
		Success bool `json:"success"`
		Result  struct { Account struct { ID string `json:"id"` } `json:"account"` } `json:"result"`
	}
	json.NewDecoder(resp.Body).Decode(&res)
	if !res.Success || res.Result.Account.ID == "" {
		return "", fmt.Errorf("não foi possível obter Account ID")
	}
	return res.Result.Account.ID, nil
}

// 4. Listar Destinos
func cfGetVerifiedDestinations(cfg Config, accountID string) ([]Destination, error) {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/email/routing/addresses", accountID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+cfg.CFToken)
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil { return nil, err }
	defer resp.Body.Close()

	var res struct {
		Success bool `json:"success"`
		Result  []Destination `json:"result"`
	}
	json.NewDecoder(resp.Body).Decode(&res)
	if !res.Success { return nil, fmt.Errorf("erro ao listar emails") }

	return res.Result, nil
}

// 5. Criar Destino (Novo Email Real)
func cfCreateDestination(cfg Config, accountID, email string) error {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/email/routing/addresses", accountID)
	payload := map[string]string{"email": email}
	body, _ := json.Marshal(payload)
	
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+cfg.CFToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil { return err }
	defer resp.Body.Close()

	var res struct {
		Success bool `json:"success"`
		Errors []struct{ Message string `json:"message"` } `json:"errors"`
	}
	json.NewDecoder(resp.Body).Decode(&res)
	
	if !res.Success {
		if len(res.Errors) > 0 { return fmt.Errorf("%s", res.Errors[0].Message) }
		return fmt.Errorf("erro ao adicionar destino")
	}
	return nil
}

// 6. Deletar Destino
func cfDeleteDestination(cfg Config, accountID, destID string) error {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/email/routing/addresses/%s", accountID, destID)
	req, _ := http.NewRequest("DELETE", url, nil)
	req.Header.Set("Authorization", "Bearer "+cfg.CFToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil { return err }
	defer resp.Body.Close()

	if resp.StatusCode != 200 { return fmt.Errorf("erro ao deletar (status %d)", resp.StatusCode) }
	return nil
}