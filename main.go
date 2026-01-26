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
	ID    int    `json:"id"`
	Email string `json:"email"`
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
	http.HandleFunc("/api/destinations", handleDestinations) // Novo: Gerenciar lista de emails
	http.HandleFunc("/api/create", handleCreate)
	http.HandleFunc("/api/active", handleListActive)
	http.HandleFunc("/api/history", handleHistory)
	http.HandleFunc("/api/delete", handleDelete)

	addr := ":" + port
	fmt.Printf("🚀 Sistema Cloudflare Mail v2 rodando em http://localhost%s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

// --- Banco de Dados ---

func initDB() {
	var err error
	db, err = sql.Open("sqlite", "data.db")
	if err != nil {
		log.Fatal(err)
	}

	// Schema Atualizado
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS config (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			cf_token TEXT,
			zone_id TEXT,
			domain TEXT
		);
		CREATE TABLE IF NOT EXISTS destinations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT UNIQUE
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

	// GET
	maskedCfg := currentCfg
	if len(currentCfg.CFToken) > 0 {
		maskedCfg.CFToken = strings.Repeat("*", len(currentCfg.CFToken))
	} else {
		maskedCfg.CFToken = ""
	}
	json.NewEncoder(w).Encode(maskedCfg)
}

// Handler para gerenciar múltiplos destinos
func handleDestinations(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var d Destination
		json.NewDecoder(r.Body).Decode(&d)
		if d.Email == "" {
			http.Error(w, "Email invalido", 400)
			return
		}
		_, err := db.Exec("INSERT INTO destinations (email) VALUES (?)", d.Email)
		if err != nil {
			http.Error(w, "Erro ao adicionar (duplicado?)", 500)
			return
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method == http.MethodDelete {
		id := r.URL.Query().Get("id")
		db.Exec("DELETE FROM destinations WHERE id = ?", id)
		w.WriteHeader(http.StatusOK)
		return
	}

	// GET
	rows, _ := db.Query("SELECT id, email FROM destinations ORDER BY id DESC")
	defer rows.Close()
	var list []Destination
	for rows.Next() {
		var d Destination
		rows.Scan(&d.ID, &d.Email)
		list = append(list, d)
	}
	if list == nil {
		list = []Destination{}
	}
	json.NewEncoder(w).Encode(list)
}

func handleCreate(w http.ResponseWriter, r *http.Request) {
	cfg, err := getConfig()
	if err != nil {
		http.Error(w, "Configure o sistema primeiro!", 400)
		return
	}

	// LER DESTINO DA REQUISIÇÃO
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Destination == "" {
		http.Error(w, "Destino obrigatório", 400)
		return
	}

	// Gerar Nome
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

	// Criar no Cloudflare usando o destino escolhido
	ruleID, err := cfCreateRule(cfg, alias, req.Destination)
	if err != nil {
		http.Error(w, "Erro Cloudflare: "+err.Error(), 500)
		return
	}

	// Salvar no DB com o destino
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

// --- Cloudflare ---

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
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var res struct {
		Success bool `json:"success"`
		Result  struct{ ID string `json:"id"` } `json:"result"`
		Errors  []struct{ Message string `json:"message"` } `json:"errors"`
	}
	json.NewDecoder(resp.Body).Decode(&res)
	if !res.Success {
		if len(res.Errors) > 0 {
			return "", fmt.Errorf("%s", res.Errors[0].Message)
		}
		return "", fmt.Errorf("erro desconhecido CF")
	}
	return res.Result.ID, nil
}

func cfDeleteRule(cfg Config, id string) {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/email/routing/rules/%s", cfg.ZoneID, id)
	req, _ := http.NewRequest("DELETE", url, nil)
	req.Header.Set("Authorization", "Bearer "+cfg.CFToken)
	req.Header.Set("Content-Type", "application/json")
	http.DefaultClient.Do(req)
}