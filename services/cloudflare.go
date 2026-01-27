package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"tempmail/models"
)

func CFCreateRule(cfg models.Config, email, destination string) (string, error) {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/email/routing/rules", cfg.ZoneID)
	payload := map[string]interface{}{
		"enabled": true, 
        "name": "Temp: " + email,
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
		Result  struct { ID string `json:"id"` } `json:"result"`
		Errors []struct { Message string `json:"message"` } `json:"errors"`
	}
	json.NewDecoder(resp.Body).Decode(&res)
	if !res.Success {
		if len(res.Errors) > 0 { return "", fmt.Errorf("%s", res.Errors[0].Message) }
		return "", fmt.Errorf("erro CF create rule")
	}
	return res.Result.ID, nil
}

func CFDeleteRule(cfg models.Config, id string) {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/email/routing/rules/%s", cfg.ZoneID, id)
	req, _ := http.NewRequest("DELETE", url, nil)
	req.Header.Set("Authorization", "Bearer "+cfg.CFToken)
	req.Header.Set("Content-Type", "application/json")
	http.DefaultClient.Do(req)
}

func CFGetAccountID(cfg models.Config) (string, error) {
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
	if !res.Success || res.Result.Account.ID == "" { return "", fmt.Errorf("não foi possível obter Account ID") }
	return res.Result.Account.ID, nil
}

func CFGetVerifiedDestinations(cfg models.Config, accountID string) ([]models.Destination, error) {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/email/routing/addresses", accountID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+cfg.CFToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil { return nil, err }
	defer resp.Body.Close()

	var res struct {
		Success bool `json:"success"`
		Result  []models.Destination `json:"result"`
	}
	json.NewDecoder(resp.Body).Decode(&res)
	if !res.Success { return nil, fmt.Errorf("erro ao listar emails") }
	return res.Result, nil
}

func CFCreateDestination(cfg models.Config, accountID, email string) error {
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
		Errors  []struct { Message string `json:"message"` } `json:"errors"`
	}
	json.NewDecoder(resp.Body).Decode(&res)
	if !res.Success {
		if len(res.Errors) > 0 { return fmt.Errorf("%s", res.Errors[0].Message) }
		return fmt.Errorf("erro ao adicionar destino")
	}
	return nil
}

func CFDeleteDestination(cfg models.Config, accountID, destID string) error {
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