package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/joho/godotenv"
)

// --- Estruturas de Dados ---

type EmailRule struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

type CFMatcher struct {
	Type  string `json:"type"`
	Field string `json:"field"`
	Value string `json:"value"`
}

type CFAction struct {
	Type  string   `json:"type"`
	Value []string `json:"value"`
}

type CFRequest struct {
	Actions  []CFAction  `json:"actions"`
	Matchers []CFMatcher `json:"matchers"`
	Enabled  bool        `json:"enabled"`
	Name     string      `json:"name"`
}

type CFResponse struct {
	Success bool `json:"success"`
	Result  struct {
		ID string `json:"id"`
	} `json:"result"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// --- Gerenciador de Estado ---

type Manager struct {
	mu     sync.Mutex
	timers map[string]*time.Timer
	emails map[string]EmailRule
}

var mgr = &Manager{
	timers: make(map[string]*time.Timer),
	emails: make(map[string]EmailRule),
}

// --- Listas de Palavras (Sem acentos para compatibilidade de email) ---
var adjetivos = []string{
	"cansado", "calvo", "radioativo", "humilde", "furioso",
	"suspeito", "duvidoso", "crocante", "quase-rico", "lendario",
	"misterioso", "caotico", "triste", "iludido", "blindado",
}

var substantivos = []string{
	"boleto", "estagiario", "capivara", "gambiarra", "tijolo",
	"hacker", "pastel", "uno-com-escada", "coach", "cafe",
	"servidor", "bug", "golpe", "primo", "vaxco",
}

// --- Configurações ---
var (
	cfToken     string
	cfZoneID    string
	cfDomain    string
	destination string
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Arquivo .env não encontrado, assumindo variáveis de ambiente.")
	}
	
	cfToken = os.Getenv("TOKEN_CLOUDFLARE")
	cfZoneID = os.Getenv("ZONE_ID")
	cfDomain = os.Getenv("DOMAIN")
	destination = os.Getenv("EMAIL_DESTINO")
	
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	if cfToken == "" || cfZoneID == "" || destination == "" {
		log.Fatal("ERRO: Variáveis de ambiente incompletas.")
	}

	http.Handle("/", http.FileServer(http.Dir("./static")))
	http.HandleFunc("/api/list", handleList)
	http.HandleFunc("/api/create", handleCreate)
	http.HandleFunc("/api/delete", handleDelete)

	addr := ":" + port
	fmt.Printf("🚀 Dashboard rodando em http://localhost%s\n", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}

// --- Handlers HTTP ---

func handleList(w http.ResponseWriter, r *http.Request) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	list := make([]EmailRule, 0, len(mgr.emails))
	for _, e := range mgr.emails {
		list = append(list, e)
	}
	json.NewEncoder(w).Encode(list)
}

func handleCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. Gerar Nome Engraçado
	nomeEmail := gerarNomeEngracado()
	alias := fmt.Sprintf("%s@%s", nomeEmail, cfDomain)

	// 2. Criar no Cloudflare
	ruleID, err := cfCreateRule(alias)
	if err != nil {
		log.Printf("Erro ao criar regra no CF: %v", err)
		http.Error(w, "Erro ao criar regra no Cloudflare", http.StatusInternalServerError)
		return
	}

	// 3. Timer de 5 min
	expiration := time.Now().Add(5 * time.Minute)
	rule := EmailRule{
		ID:        ruleID,
		Email:     alias,
		CreatedAt: time.Now(),
		ExpiresAt: expiration,
	}

	mgr.mu.Lock()
	mgr.emails[ruleID] = rule
	mgr.timers[ruleID] = time.AfterFunc(5*time.Minute, func() {
		log.Printf("⏰ Auto-deletando %s", alias)
		deleteRoutine(ruleID)
	})
	mgr.mu.Unlock()

	json.NewEncoder(w).Encode(rule)
}

func handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "ID required", http.StatusBadRequest)
		return
	}

	if err := deleteRoutine(id); err != nil {
		log.Printf("Erro ao deletar regra %s: %v", id, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// --- Lógica Auxiliar ---

func gerarNomeEngracado() string {
	// Pega um índice aleatório seguro para adjetivos
	nAdj, _ := rand.Int(rand.Reader, big.NewInt(int64(len(adjetivos))))
	// Pega um índice aleatório seguro para substantivos
	nSub, _ := rand.Int(rand.Reader, big.NewInt(int64(len(substantivos))))
	// Pega um número aleatório de 0 a 999 para evitar colisões
	nNum, _ := rand.Int(rand.Reader, big.NewInt(1000))

	adj := adjetivos[nAdj.Int64()]
	sub := substantivos[nSub.Int64()]

	return fmt.Sprintf("%s-%s-%d", sub, adj, nNum.Int64())
}

func deleteRoutine(id string) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	if _, exists := mgr.emails[id]; !exists {
		return nil
	}

	if t, ok := mgr.timers[id]; ok {
		t.Stop()
		delete(mgr.timers, id)
	}

	if err := cfDeleteRule(id); err != nil {
		log.Printf("Aviso: Falha ao deletar no Cloudflare: %v", err)
	}

	delete(mgr.emails, id)
	return nil
}

// --- Integração Cloudflare ---

func cfCreateRule(email string) (string, error) {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/email/routing/rules", cfZoneID)
	
	payload := CFRequest{
		Enabled: true,
		Name:    "Temp: " + email,
		Matchers: []CFMatcher{
			{Type: "literal", Field: "to", Value: email},
		},
		Actions: []CFAction{
			{Type: "forward", Value: []string{destination}},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil { return "", err }
	
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil { return "", err }

	req.Header.Set("Authorization", "Bearer "+cfToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil { return "", err }
	defer resp.Body.Close()

	var res CFResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil { return "", err }
	
	if !res.Success {
		msg := "Unknown API Error"
		if len(res.Errors) > 0 { msg = res.Errors[0].Message }
		return "", fmt.Errorf("Cloudflare API Error: %s", msg)
	}
	return res.Result.ID, nil
}

func cfDeleteRule(id string) error {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/email/routing/rules/%s", cfZoneID, id)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil { return err }

	req.Header.Set("Authorization", "Bearer "+cfToken)
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil { return err }
	defer resp.Body.Close()

	if resp.StatusCode != 200 { return fmt.Errorf("status code: %d", resp.StatusCode) }
	return nil
}