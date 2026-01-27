package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"tempmail/database"
	"tempmail/models"
	"tempmail/services"
)

var (
	adjetivos      = []string{"cansado", "calvo", "radioativo", "humilde", "furioso", "suspeito", "duvidoso", "crocante", "quase-rico", "lendario", "misterioso", "caotico", "triste", "iludido", "blindado", "agiota", "nutella", "raiz", "toxico", "quase-senior"}
	substantivos   = []string{"boleto", "estagiario", "capivara", "gambiarra", "tijolo", "hacker", "pastel", "uno-com-escada", "coach", "cafe", "servidor", "bug", "golpe", "primo", "vaxco", "lider-tecnico", "git-blame", "deploy", "backup", "junior"}
	coresTags      = []string{"#ef4444", "#f97316", "#f59e0b", "#84cc16", "#10b981", "#06b6d4", "#3b82f6", "#6366f1", "#8b5cf6", "#d946ef", "#f43f5e"}
	activeTimers   = make(map[string]*time.Timer)
	timerMu        sync.Mutex
)

// --- Auxiliares Internos ---

func getConfig() (models.Config, error) {
	var c models.Config
	err := database.DB.QueryRow("SELECT cf_token, zone_id, domain FROM config WHERE id = 1").Scan(&c.CFToken, &c.ZoneID, &c.Domain)
	if c.CFToken == "" { return c, fmt.Errorf("no config") }
	return c, err
}

func startTimer(id, email string, cfg models.Config) {
	timerMu.Lock()
	defer timerMu.Unlock()
	activeTimers[id] = time.AfterFunc(5*time.Minute, func() {
		log.Printf("⏰ Expirou: %s", email)
		services.CFDeleteRule(cfg, id)
		database.DB.Exec("UPDATE emails SET active = 0 WHERE id = ?", id)
		timerMu.Lock()
		delete(activeTimers, id)
		timerMu.Unlock()
	})
}

func gerarNomeEngracado() string {
	nAdj, _ := rand.Int(rand.Reader, big.NewInt(int64(len(adjetivos))))
	nSub, _ := rand.Int(rand.Reader, big.NewInt(int64(len(substantivos))))
	nNum, _ := rand.Int(rand.Reader, big.NewInt(1000))
	return fmt.Sprintf("%s-%s-%d", substantivos[nSub.Int64()], adjetivos[nAdj.Int64()], nNum.Int64())
}

func sendRowsWithTags(w http.ResponseWriter, rows *sql.Rows) {
	var list []models.EmailEntry
	if rows != nil {
		for rows.Next() {
			var e models.EmailEntry
			rows.Scan(&e.ID, &e.Email, &e.Destination, &e.CreatedAt, &e.Active, &e.Pinned)
			tagRows, err := database.DB.Query(`
				SELECT t.id, t.name, t.color 
				FROM tags t 
				JOIN email_tags et ON t.id = et.tag_id 
				WHERE et.email_id = ?`, e.ID)
			if err == nil {
				var tags []models.Tag
				for tagRows.Next() {
					var t models.Tag
					tagRows.Scan(&t.ID, &t.Name, &t.Color)
					tags = append(tags, t)
				}
				tagRows.Close()
				e.Tags = tags
			}
			if e.Tags == nil { e.Tags = []models.Tag{} }
			list = append(list, e)
		}
	}
	if list == nil { list = []models.EmailEntry{} }
	json.NewEncoder(w).Encode(list)
}

// --- Handlers Públicos ---

func HandleTags(w http.ResponseWriter, r *http.Request) {
	rows, err := database.DB.Query("SELECT id, name, color FROM tags ORDER BY name")
	if err != nil { http.Error(w, err.Error(), 500); return }
	defer rows.Close()
	var tags []models.Tag
	for rows.Next() {
		var t models.Tag
		rows.Scan(&t.ID, &t.Name, &t.Color)
		tags = append(tags, t)
	}
	if tags == nil { tags = []models.Tag{} }
	json.NewEncoder(w).Encode(tags)
}

func HandlePin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { http.Error(w, "Method not allowed", 405); return }
	var req models.PinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { http.Error(w, "JSON inválido", 400); return }
	cfg, err := getConfig()
	if err != nil { http.Error(w, "Erro config", 500); return }

	database.DB.Exec("UPDATE emails SET pinned = ? WHERE id = ?", req.Pinned, req.ID)
	timerMu.Lock()
	defer timerMu.Unlock()

	if req.Pinned {
		if t, ok := activeTimers[req.ID]; ok {
			t.Stop()
			delete(activeTimers, req.ID)
		}
	} else {
		if _, ok := activeTimers[req.ID]; !ok {
			activeTimers[req.ID] = time.AfterFunc(5*time.Minute, func() {
				services.CFDeleteRule(cfg, req.ID)
				database.DB.Exec("UPDATE emails SET active = 0 WHERE id = ?", req.ID)
				timerMu.Lock()
				delete(activeTimers, req.ID)
				timerMu.Unlock()
			})
			database.DB.Exec("UPDATE emails SET created_at = ? WHERE id = ?", time.Now(), req.ID)
		}
	}
	w.WriteHeader(http.StatusOK)
}

func HandleConfig(w http.ResponseWriter, r *http.Request) {
	var currentCfg models.Config
	database.DB.QueryRow("SELECT cf_token, zone_id, domain FROM config WHERE id = 1").Scan(&currentCfg.CFToken, &currentCfg.ZoneID, &currentCfg.Domain)

	if r.Method == http.MethodPost {
		var newCfg models.Config
		json.NewDecoder(r.Body).Decode(&newCfg)
		finalToken := newCfg.CFToken
		if len(currentCfg.CFToken) > 0 && newCfg.CFToken == strings.Repeat("*", len(currentCfg.CFToken)) {
			finalToken = currentCfg.CFToken
		}
		database.DB.Exec(`
			INSERT INTO config (id, cf_token, zone_id, domain) 
			VALUES (1, ?, ?, ?) 
			ON CONFLICT(id) DO UPDATE SET cf_token=excluded.cf_token, zone_id=excluded.zone_id, domain=excluded.domain
		`, finalToken, newCfg.ZoneID, newCfg.Domain)
		w.WriteHeader(http.StatusOK)
		return
	}
	maskedCfg := currentCfg
	if len(currentCfg.CFToken) > 0 { maskedCfg.CFToken = strings.Repeat("*", len(currentCfg.CFToken)) }
	json.NewEncoder(w).Encode(maskedCfg)
}

func HandleDestinations(w http.ResponseWriter, r *http.Request) {
	cfg, err := getConfig()
	if err != nil { http.Error(w, "Config required", 400); return }
	accID, err := services.CFGetAccountID(cfg)
	if err != nil { http.Error(w, err.Error(), 500); return }

	switch r.Method {
	case http.MethodGet:
		dests, _ := services.CFGetVerifiedDestinations(cfg, accID)
		json.NewEncoder(w).Encode(dests)
	case http.MethodPost:
		var req struct{ Email string `json:"email"` }
		json.NewDecoder(r.Body).Decode(&req)
		services.CFCreateDestination(cfg, accID, req.Email)
		w.WriteHeader(http.StatusOK)
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		services.CFDeleteDestination(cfg, accID, id)
		w.WriteHeader(http.StatusOK)
	}
}

func HandleCreate(w http.ResponseWriter, r *http.Request) {
	cfg, err := getConfig()
	if err != nil { http.Error(w, "Config required", 400); return }
	var req models.CreateRequest
	json.NewDecoder(r.Body).Decode(&req)

	alias := req.Email
	if alias == "" {
		alias = fmt.Sprintf("%s@%s", gerarNomeEngracado(), cfg.Domain)
	}

	ruleID, err := services.CFCreateRule(cfg, alias, req.Destination)
	if err != nil { http.Error(w, err.Error(), 500); return }

	database.DB.Exec(`
		INSERT INTO emails (id, email, destination, created_at, active, pinned) 
		VALUES (?, ?, ?, ?, ?, 0)
		ON CONFLICT(email) DO UPDATE SET id=excluded.id, destination=excluded.destination, created_at=excluded.created_at, active=excluded.active, pinned=0
	`, ruleID, alias, req.Destination, time.Now(), true)

	database.DB.Exec("DELETE FROM email_tags WHERE email_id = ?", ruleID)
	for _, tagName := range req.Tags {
		tagName = strings.TrimSpace(strings.ToLower(tagName))
		if tagName == "" { continue }
		var tagID int64
		err := database.DB.QueryRow("SELECT id FROM tags WHERE name = ?", tagName).Scan(&tagID)
		if err == sql.ErrNoRows {
			idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(coresTags))))
			res, _ := database.DB.Exec("INSERT INTO tags (name, color) VALUES (?, ?)", tagName, coresTags[idx.Int64()])
			tagID, _ = res.LastInsertId()
		}
		database.DB.Exec("INSERT INTO email_tags (email_id, tag_id) VALUES (?, ?)", ruleID, tagID)
	}

	startTimer(ruleID, alias, cfg)
	json.NewEncoder(w).Encode(map[string]string{"id": ruleID, "email": alias})
}

func HandleCheck(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	var active bool
	err := database.DB.QueryRow("SELECT active FROM emails WHERE email = ?", email).Scan(&active)
	if err == sql.ErrNoRows { json.NewEncoder(w).Encode(map[string]bool{"exists": false}); return }
	json.NewEncoder(w).Encode(map[string]bool{"exists": true, "active": active})
}

func HandleListActive(w http.ResponseWriter, r *http.Request) {
	rows, _ := database.DB.Query("SELECT id, email, destination, created_at, active, pinned FROM emails WHERE active = 1 ORDER BY pinned DESC, created_at DESC")
	defer rows.Close()
	sendRowsWithTags(w, rows)
}

func HandleHistory(w http.ResponseWriter, r *http.Request) {
	rows, _ := database.DB.Query("SELECT id, email, destination, created_at, active, pinned FROM emails ORDER BY created_at DESC")
	defer rows.Close()
	sendRowsWithTags(w, rows)
}

func HandleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	cfg, _ := getConfig()
	services.CFDeleteRule(cfg, id)
	database.DB.Exec("UPDATE emails SET active = 0 WHERE id = ?", id)
	timerMu.Lock()
	if t, ok := activeTimers[id]; ok { t.Stop(); delete(activeTimers, id) }
	timerMu.Unlock()
	w.WriteHeader(http.StatusOK)
}