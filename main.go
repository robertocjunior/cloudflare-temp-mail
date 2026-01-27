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
	Tag      string `json:"tag"`
	Email    string `json:"email"`
	Verified string `json:"verified,omitempty"`
}

type Tag struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type EmailEntry struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	Destination string    `json:"destination"`
	CreatedAt   time.Time `json:"created_at"`
	Active      bool      `json:"active"`
	Pinned      bool      `json:"pinned"`
	Tags        []Tag     `json:"tags"` // Novo campo
}

type CreateRequest struct {
	Destination string   `json:"destination"`
	Email       string   `json:"email,omitempty"`
	Tags        []string `json:"tags,omitempty"` // Novo campo
}

type PinRequest struct {
	ID     string `json:"id"`
	Pinned bool   `json:"pinned"`
}

// --- Listas de Nomes ---
var adjetivos = []string{"cansado", "calvo", "radioativo", "humilde", "furioso", "suspeito", "duvidoso", "crocante", "quase-rico", "lendario", "misterioso", "caotico", "triste", "iludido", "blindado", "agiota", "nutella", "raiz", "toxico", "quase-senior"}
var substantivos = []string{"boleto", "estagiario", "capivara", "gambiarra", "tijolo", "hacker", "pastel", "uno-com-escada", "coach", "cafe", "servidor", "bug", "golpe", "primo", "vaxco", "lider-tecnico", "git-blame", "deploy", "backup", "junior"}
var coresTags = []string{"#ef4444", "#f97316", "#f59e0b", "#84cc16", "#10b981", "#06b6d4", "#3b82f6", "#6366f1", "#8b5cf6", "#d946ef", "#f43f5e"}

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
	http.HandleFunc("/api/check", handleCheck)
	http.HandleFunc("/api/create", handleCreate)
	http.HandleFunc("/api/pin", handlePin)
	http.HandleFunc("/api/active", handleListActive)
	http.HandleFunc("/api/history", handleHistory)
	http.HandleFunc("/api/delete", handleDelete)
	http.HandleFunc("/api/tags", handleTags) // Nova rota

	addr := ":" + port
	fmt.Printf("🚀 Sistema Cloudflare Mail v6.1 (Tags) rodando em http://localhost%s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

// --- Banco de Dados ---

func initDB() {
	var err error
	db, err = sql.Open("sqlite", "data.db")
	if err != nil {
		log.Fatal(err)
	}

	// Cria tabelas base
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
			active BOOLEAN,
			pinned BOOLEAN DEFAULT 0
		);
		CREATE TABLE IF NOT EXISTS tags (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE,
			color TEXT
		);
		CREATE TABLE IF NOT EXISTS email_tags (
			email_id TEXT,
			tag_id INTEGER,
			PRIMARY KEY (email_id, tag_id),
			FOREIGN KEY(email_id) REFERENCES emails(id),
			FOREIGN KEY(tag_id) REFERENCES tags(id)
		);
	`)
	if err != nil {
		log.Fatal("Erro ao migrar DB:", err)
	}

	// Migração manual de segurança
	db.Exec("ALTER TABLE emails ADD COLUMN pinned BOOLEAN DEFAULT 0;")
}

// --- Handlers ---

// Retorna lista de tags existentes para autocomplete
func handleTags(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, name, color FROM tags ORDER BY name")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var tags []Tag
	for rows.Next() {
		var t Tag
		rows.Scan(&t.ID, &t.Name, &t.Color)
		tags = append(tags, t)
	}
	if tags == nil {
		tags = []Tag{}
	}
	json.NewEncoder(w).Encode(tags)
}

func handlePin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}

	var req PinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}

	cfg, err := getConfig()
	if err != nil {
		http.Error(w, "Erro config", 500)
		return
	}

	_, err = db.Exec("UPDATE emails SET pinned = ? WHERE id = ?", req.Pinned, req.ID)
	if err != nil {
		http.Error(w, "Erro ao atualizar DB", 500)
		return
	}

	timerMu.Lock()
	defer timerMu.Unlock()

	if req.Pinned {
		if t, ok := activeTimers[req.ID]; ok {
			t.Stop()
			delete(activeTimers, req.ID)
			log.Printf("📌 Email fixado: %s", req.ID)
		}
	} else {
		if _, ok := activeTimers[req.ID]; !ok {
			activeTimers[req.ID] = time.AfterFunc(5*time.Minute, func() {
				log.Printf("⏰ Expirou (pós-fixação): %s", req.ID)
				cfDeleteRule(cfg, req.ID)
				db.Exec("UPDATE emails SET active = 0 WHERE id = ?", req.ID)
				timerMu.Lock()
				delete(activeTimers, req.ID)
				timerMu.Unlock()
			})
			db.Exec("UPDATE emails SET created_at = ? WHERE id = ?", time.Now(), req.ID)
		}
	}

	w.WriteHeader(http.StatusOK)
}

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

func handleDestinations(w http.ResponseWriter, r *http.Request) {
	cfg, err := getConfig()
	if err != nil {
		http.Error(w, "Configure o sistema primeiro", 400)
		return
	}
	accountID, err := cfGetAccountID(cfg)
	if err != nil {
		http.Error(w, "Erro Account ID: "+err.Error(), 500)
		return
	}

	if r.Method == http.MethodGet {
		dests, err := cfGetVerifiedDestinations(cfg, accountID)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		json.NewEncoder(w).Encode(dests)
		return
	}

	if r.Method == http.MethodPost {
		var req struct {
			Email string `json:"email"`
		}
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

	if r.Method == http.MethodDelete {
		destID := r.URL.Query().Get("id")
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

func handleCheck(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	if email == "" {
		http.Error(w, "Email required", 400)
		return
	}

	var exists bool
	var active bool
	err := db.QueryRow("SELECT 1, active FROM emails WHERE email = ?", email).Scan(&exists, &active)

	if err == sql.ErrNoRows {
		json.NewEncoder(w).Encode(map[string]bool{"exists": false})
		return
	}

	json.NewEncoder(w).Encode(map[string]bool{"exists": true, "active": active})
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
	if req.Email != "" {
		alias = req.Email
	} else {
		for i := 0; i < 10; i++ {
			candidato := fmt.Sprintf("%s@%s", gerarNomeEngracado(), cfg.Domain)
			if !emailExists(candidato) {
				alias = candidato
				break
			}
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

	// 1. Inserir Email
	_, err = db.Exec(`
		INSERT INTO emails (id, email, destination, created_at, active, pinned) 
		VALUES (?, ?, ?, ?, ?, 0)
		ON CONFLICT(email) DO UPDATE SET 
			id=excluded.id, 
			destination=excluded.destination, 
			created_at=excluded.created_at, 
			active=excluded.active,
			pinned=0
	`, ruleID, alias, req.Destination, time.Now(), true)

	if err != nil {
		log.Println("Erro DB Email:", err)
		http.Error(w, "Erro DB", 500)
		return
	}

	// 2. Processar Tags
	// Limpa tags antigas se for uma recriação (ON CONFLICT acima)
	db.Exec("DELETE FROM email_tags WHERE email_id = ?", ruleID)

	for _, tagName := range req.Tags {
		tagName = strings.TrimSpace(tagName)
		if tagName == "" {
			continue
		}

		// Busca ID da tag, ou cria nova se não existir
		var tagID int64
		var tagColor string
		err := db.QueryRow("SELECT id, color FROM tags WHERE name = ?", tagName).Scan(&tagID, &tagColor)

		if err == sql.ErrNoRows {
			// Gera cor aleatoria
			idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(coresTags))))
			newColor := coresTags[idx.Int64()]
			res, err := db.Exec("INSERT INTO tags (name, color) VALUES (?, ?)", tagName, newColor)
			if err != nil {
				continue
			}
			tagID, _ = res.LastInsertId()
		} else if err != nil {
			continue
		}

		// Linka Tag ao Email
		db.Exec("INSERT INTO email_tags (email_id, tag_id) VALUES (?, ?)", ruleID, tagID)
	}

	startTimer(ruleID, alias, cfg)
	json.NewEncoder(w).Encode(map[string]string{"id": ruleID, "email": alias})
}

func handleListActive(w http.ResponseWriter, r *http.Request) {
	rows, _ := db.Query("SELECT id, email, destination, created_at, active, pinned FROM emails WHERE active = 1 ORDER BY pinned DESC, created_at DESC")
	if rows != nil {
		defer rows.Close()
	}
	sendRowsWithTags(w, rows)
}

func handleHistory(w http.ResponseWriter, r *http.Request) {
	rows, _ := db.Query("SELECT id, email, destination, created_at, active, pinned FROM emails ORDER BY created_at DESC")
	if rows != nil {
		defer rows.Close()
	}
	sendRowsWithTags(w, rows)
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
	// Nota: não deletamos as tags do histórico

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

func sendRowsWithTags(w http.ResponseWriter, rows *sql.Rows) {
	var list []EmailEntry
	if rows != nil {
		for rows.Next() {
			var e EmailEntry
			rows.Scan(&e.ID, &e.Email, &e.Destination, &e.CreatedAt, &e.Active, &e.Pinned)
			
			// Busca Tags para este email
			tagRows, err := db.Query(`
				SELECT t.id, t.name, t.color 
				FROM tags t 
				JOIN email_tags et ON t.id = et.tag_id 
				WHERE et.email_id = ?`, e.ID)
			
			if err == nil {
				var tags []Tag
				for tagRows.Next() {
					var t Tag
					tagRows.Scan(&t.ID, &t.Name, &t.Color)
					tags = append(tags, t)
				}
				tagRows.Close()
				e.Tags = tags
			}
			if e.Tags == nil { e.Tags = []Tag{} } // Garante JSON array vazio [] em vez de null

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
		Result  struct {
			ID string `json:"id"`
		} `json:"result"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	json.NewDecoder(resp.Body).Decode(&res)
	if !res.Success {
		if len(res.Errors) > 0 {
			return "", fmt.Errorf("%s", res.Errors[0].Message)
		}
		return "", fmt.Errorf("erro CF create rule")
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

func cfGetAccountID(cfg Config) (string, error) {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s", cfg.ZoneID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+cfg.CFToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var res struct {
		Success bool `json:"success"`
		Result  struct {
			Account struct {
				ID string `json:"id"`
			} `json:"account"`
		} `json:"result"`
	}
	json.NewDecoder(resp.Body).Decode(&res)
	if !res.Success || res.Result.Account.ID == "" {
		return "", fmt.Errorf("não foi possível obter Account ID")
	}
	return res.Result.Account.ID, nil
}

func cfGetVerifiedDestinations(cfg Config, accountID string) ([]Destination, error) {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/email/routing/addresses", accountID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+cfg.CFToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var res struct {
		Success bool `json:"success"`
		Result  []Destination `json:"result"`
	}
	json.NewDecoder(resp.Body).Decode(&res)
	if !res.Success {
		return nil, fmt.Errorf("erro ao listar emails")
	}
	return res.Result, nil
}

func cfCreateDestination(cfg Config, accountID, email string) error {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/email/routing/addresses", accountID)
	payload := map[string]string{"email": email}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+cfg.CFToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var res struct {
		Success bool `json:"success"`
		Errors  []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	json.NewDecoder(resp.Body).Decode(&res)

	if !res.Success {
		if len(res.Errors) > 0 {
			return fmt.Errorf("%s", res.Errors[0].Message)
		}
		return fmt.Errorf("erro ao adicionar destino")
	}
	return nil
}

func cfDeleteDestination(cfg Config, accountID, destID string) error {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/email/routing/addresses/%s", accountID, destID)
	req, _ := http.NewRequest("DELETE", url, nil)
	req.Header.Set("Authorization", "Bearer "+cfg.CFToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("erro ao deletar (status %d)", resp.StatusCode)
	}
	return nil
}