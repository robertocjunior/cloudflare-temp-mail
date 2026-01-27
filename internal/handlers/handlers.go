package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"tempmail/internal/database"
	"tempmail/internal/models"
	"tempmail/internal/services"
	"time"
)

var (
	adjetivos    = []string{"cansado", "calvo", "radioativo", "humilde", "furioso", "suspeito", "duvidoso", "crocante", "quase-rico", "lendario", "misterioso", "caotico", "triste", "iludido", "blindado", "agiota", "nutella", "raiz", "toxico", "quase-senior"}
	substantivos = []string{"boleto", "estagiario", "capivara", "gambiarra", "tijolo", "hacker", "pastel", "uno-com-escada", "coach", "cafe", "servidor", "bug", "golpe", "primo", "vaxco", "lider-tecnico", "git-blame", "deploy", "backup", "junior"}
	coresTags    = []string{"#ef4444", "#f97316", "#f59e0b", "#84cc16", "#10b981", "#06b6d4", "#3b82f6", "#6366f1", "#8b5cf6", "#d946ef", "#f43f5e"}
	activeTimers = make(map[string]*time.Timer)
	timerMu      sync.Mutex
)

func HandleTags(w http.ResponseWriter, r *http.Request) {
	rows, err := database.DB.Query("SELECT id, name, color FROM tags ORDER BY name")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var tags []models.Tag
	for rows.Next() {
		var t models.Tag
		rows.Scan(&t.ID, &t.Name, &t.Color)
		tags = append(tags, t)
	}
	if tags == nil {
		tags = []models.Tag{}
	}
	json.NewEncoder(w).Encode(tags)
}

func HandlePin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}

	var req models.PinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "JSON inválido", 400)
		return
	}

	cfg, err := database.GetConfig()
	if err != nil {
		http.Error(w, "Erro config", 500)
		return
	}

	_, err = database.DB.Exec("UPDATE emails SET pinned = ? WHERE id = ?", req.Pinned, req.ID)
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
		}
	} else {
		if _, ok := activeTimers[req.ID]; !ok {
			activeTimers[req.ID] = time.AfterFunc(5*time.Minute, func() {
				services.CfDeleteRule(cfg, req.ID)
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
	row := database.DB.QueryRow("SELECT cf_token, zone_id, domain FROM config WHERE id = 1")
	row.Scan(&currentCfg.CFToken, &currentCfg.ZoneID, &currentCfg.Domain)

	if r.Method == http.MethodPost {
		var newCfg models.Config
		if err := json.NewDecoder(r.Body).Decode(&newCfg); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}

		finalToken := newCfg.CFToken
		if len(currentCfg.CFToken) > 0 && newCfg.CFToken == strings.Repeat("*", len(currentCfg.CFToken)) {
			finalToken = currentCfg.CFToken
		}

		_, err := database.DB.Exec(`
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

func HandleDestinations(w http.ResponseWriter, r *http.Request) {
	cfg, err := database.GetConfig()
	if err != nil {
		http.Error(w, "Configure o sistema primeiro", 400)
		return
	}
	accountID, err := services.CfGetAccountID(cfg)
	if err != nil {
		http.Error(w, "Erro Account ID: "+err.Error(), 500)
		return
	}

	if r.Method == http.MethodGet {
		dests, err := services.CfGetVerifiedDestinations(cfg, accountID)
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
		if err := services.CfCreateDestination(cfg, accountID, req.Email); err != nil {
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
		if err := services.CfDeleteDestination(cfg, accountID, destID); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Error(w, "Method not allowed", 405)
}

func HandleCheck(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	if email == "" {
		http.Error(w, "Email required", 400)
		return
	}

	var exists bool
	var active bool
	err := database.DB.QueryRow("SELECT 1, active FROM emails WHERE email = ?", email).Scan(&exists, &active)

	if err == sql.ErrNoRows {
		json.NewEncoder(w).Encode(map[string]bool{"exists": false})
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"exists": true, "active": active})
}

func HandleCreate(w http.ResponseWriter, r *http.Request) {
	cfg, err := database.GetConfig()
	if err != nil {
		http.Error(w, "Configure o sistema primeiro!", 400)
		return
	}

	var req models.CreateRequest
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
			if !database.EmailExists(candidato) {
				alias = candidato
				break
			}
		}
	}

	if alias == "" {
		http.Error(w, "Falha ao gerar nome único", 500)
		return
	}

	ruleID, err := services.CfCreateRule(cfg, alias, req.Destination)
	if err != nil {
		http.Error(w, "Erro Cloudflare: "+err.Error(), 500)
		return
	}

	_, err = database.DB.Exec(`
		INSERT INTO emails (id, email, destination, created_at, active, pinned) 
		VALUES (?, ?, ?, ?, ?, 0)
		ON CONFLICT(email) DO UPDATE SET 
			id=excluded.id, 
			destination=excluded.destination, 
			created_at=excluded.created_at, 
			active=excluded.active,
			pinned=0
	`, ruleID, alias, req.Destination, time.Now(), true)

	database.DB.Exec("DELETE FROM email_tags WHERE email_id = ?", ruleID)

	for _, tagName := range req.Tags {
		tagName = strings.TrimSpace(tagName)
		if tagName == "" {
			continue
		}
		var tagID int64
		var tagColor string
		err := database.DB.QueryRow("SELECT id, color FROM tags WHERE name = ?", tagName).Scan(&tagID, &tagColor)

		if err == sql.ErrNoRows {
			idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(coresTags))))
			newColor := coresTags[idx.Int64()]
			res, err := database.DB.Exec("INSERT INTO tags (name, color) VALUES (?, ?)", tagName, newColor)
			if err != nil {
				continue
			}
			tagID, _ = res.LastInsertId()
		} else if err != nil {
			continue
		}
		database.DB.Exec("INSERT INTO email_tags (email_id, tag_id) VALUES (?, ?)", ruleID, tagID)
	}

	startTimer(ruleID, cfg)
	json.NewEncoder(w).Encode(map[string]string{"id": ruleID, "email": alias})
}

func HandleListActive(w http.ResponseWriter, r *http.Request) {
	rows, _ := database.DB.Query("SELECT id, email, destination, created_at, active, pinned FROM emails WHERE active = 1 ORDER BY pinned DESC, created_at DESC")
	if rows != nil {
		defer rows.Close()
	}
	sendRowsWithTags(w, rows)
}

func HandleHistory(w http.ResponseWriter, r *http.Request) {
	rows, _ := database.DB.Query("SELECT id, email, destination, created_at, active, pinned FROM emails ORDER BY created_at DESC")
	if rows != nil {
		defer rows.Close()
	}
	sendRowsWithTags(w, rows)
}

func HandleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	cfg, err := database.GetConfig()
	if err != nil {
		http.Error(w, "Erro config", 500)
		return
	}

	services.CfDeleteRule(cfg, id)
	database.DB.Exec("UPDATE emails SET active = 0 WHERE id = ?", id)

	timerMu.Lock()
	if t, ok := activeTimers[id]; ok {
		t.Stop()
		delete(activeTimers, id)
	}
	timerMu.Unlock()
	w.WriteHeader(http.StatusOK)
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
	if list == nil {
		list = []models.EmailEntry{}
	}
	json.NewEncoder(w).Encode(list)
}

func startTimer(id string, cfg models.Config) {
	timerMu.Lock()
	activeTimers[id] = time.AfterFunc(5*time.Minute, func() {
		services.CfDeleteRule(cfg, id)
		database.DB.Exec("UPDATE emails SET active = 0 WHERE id = ?", id)
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