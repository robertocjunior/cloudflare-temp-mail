package database

import (
	"database/sql"
	"log"
	"tempmail/internal/models"

	_ "github.com/glebarez/go-sqlite"
)

var DB *sql.DB

func InitDB() {
	var err error
	DB, err = sql.Open("sqlite", "data.db")
	if err != nil {
		log.Fatal(err)
	}

	_, err = DB.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE,
			password TEXT,
			full_name TEXT,
			created_at DATETIME
		);
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
}

func IsSetupDone() bool {
	var count int
	DB.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	return count > 0
}

func GetConfig() (models.Config, error) {
	var c models.Config
	err := DB.QueryRow("SELECT cf_token, zone_id, domain FROM config WHERE id = 1").Scan(&c.CFToken, &c.ZoneID, &c.Domain)
	return c, err
}

func EmailExists(email string) bool {
	var exists bool
	DB.QueryRow("SELECT EXISTS(SELECT 1 FROM emails WHERE email = ?)", email).Scan(&exists)
	return exists
}