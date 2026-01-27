package database

import (
	"database/sql"
	"log"

	_ "github.com/glebarez/go-sqlite"
)

var DB *sql.DB

func InitDB() {
	var err error
	DB, err = sql.Open("sqlite", "data.db")
	if err != nil {
		log.Fatal(err)
	}

	query := `
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
	`
	_, err = DB.Exec(query)
	if err != nil {
		log.Fatal("Erro ao migrar DB:", err)
	}
    // Garante que a coluna pinned existe para compatibilidade
    DB.Exec("ALTER TABLE emails ADD COLUMN pinned BOOLEAN DEFAULT 0;")
}