package models

import "time"

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
	Tags        []Tag     `json:"tags"`
}

type CreateRequest struct {
	Destination string   `json:"destination"`
	Email       string   `json:"email,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

type PinRequest struct {
	ID     string `json:"id"`
	Pinned bool   `json:"pinned"`
}