package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
)

type Migration struct {
	ID      int
	Name    string
	UpSQL   string
	DownSQL string
}

var migrations = []Migration{
	{ID: 1, Name: "initial_schema", UpSQL: initialSchemaSQL},
	{ID: 2, Name: "add_proxy_url", UpSQL: addProxyURLSQL},
	{ID: 3, Name: "change_id_to_string", UpSQL: changeIDToStringSQL},
	{ID: 4, Name: "add_s3_support", UpSQL: addS3SupportSQL},
	{ID: 5, Name: "add_message_history", UpSQL: addMessageHistorySQL},
	{ID: 6, Name: "add_quoted_message_id", UpSQL: addQuotedMessageIDSQL},
	{ID: 7, Name: "add_hmac_key", UpSQL: addHmacKeySQL},
	{ID: 8, Name: "add_data_json", UpSQL: addDataJsonSQL},
	{ID: 9, Name: "add_wa_contacts", UpSQL: addWAContactsSQL},
}

/* ===================== SQL BLOBS (POSTGRES) ===================== */

const initialSchemaSQL = `
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'users') THEN
        CREATE TABLE users (
            id TEXT PRIMARY KEY,
            name TEXT NOT NULL,
            token TEXT NOT NULL,
            webhook TEXT NOT NULL DEFAULT '',
            jid TEXT NOT NULL DEFAULT '',
            qrcode TEXT NOT NULL DEFAULT '',
            connected INTEGER,
            expiration INTEGER,
            events TEXT NOT NULL DEFAULT '',
            proxy_url TEXT DEFAULT ''
        );
    END IF;
END $$;
`

const addProxyURLSQL = `
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'users' AND column_name = 'proxy_url'
    ) THEN
        ALTER TABLE users ADD COLUMN proxy_url TEXT DEFAULT '';
    END IF;
END $$;
`

const changeIDToStringSQL = `
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'users' AND column_name = 'id' AND data_type = 'integer'
    ) THEN
        ALTER TABLE users ADD COLUMN new_id TEXT;
        UPDATE users SET new_id = md5(random()::text || id::text || clock_timestamp()::text);
        ALTER TABLE users DROP CONSTRAINT users_pkey;
        ALTER TABLE users DROP COLUMN id CASCADE;
        ALTER TABLE users RENAME COLUMN new_id TO id;
        ALTER TABLE users ALTER COLUMN id SET NOT NULL;
        ALTER TABLE users ADD PRIMARY KEY (id);
    END IF;
END $$;
`

const addS3SupportSQL = `
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='s3_enabled') THEN
        ALTER TABLE users ADD COLUMN s3_enabled BOOLEAN DEFAULT FALSE;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='s3_endpoint') THEN
        ALTER TABLE users ADD COLUMN s3_endpoint TEXT DEFAULT '';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='s3_region') THEN
        ALTER TABLE users ADD COLUMN s3_region TEXT DEFAULT '';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='s3_bucket') THEN
        ALTER TABLE users ADD COLUMN s3_bucket TEXT DEFAULT '';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='s3_access_key') THEN
        ALTER TABLE users ADD COLUMN s3_access_key TEXT DEFAULT '';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='s3_secret_key') THEN
        ALTER TABLE users ADD COLUMN s3_secret_key TEXT DEFAULT '';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='s3_path_style') THEN
        ALTER TABLE users ADD COLUMN s3_path_style BOOLEAN DEFAULT TRUE;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='s3_public_url') THEN
        ALTER TABLE users ADD COLUMN s3_public_url TEXT DEFAULT '';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='media_delivery') THEN
        ALTER TABLE users ADD COLUMN media_delivery TEXT DEFAULT 'base64';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='s3_retention_days') THEN
        ALTER TABLE users ADD COLUMN s3_retention_days INTEGER DEFAULT 30;
    END IF;
END $$;
`

const addMessageHistorySQL = `
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name='message_history') THEN
        CREATE TABLE message_history (
            id SERIAL PRIMARY KEY,
            user_id TEXT NOT NULL,
            chat_jid TEXT NOT NULL,
            sender_jid TEXT NOT NULL,
            message_id TEXT NOT NULL,
            timestamp TIMESTAMP NOT NULL,
            message_type TEXT NOT NULL,
            text_content TEXT,
            media_link TEXT,
            UNIQUE(user_id, message_id)
        );
        CREATE INDEX idx_message_history_user_chat_timestamp
            ON message_history (user_id, chat_jid, timestamp DESC);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name='users' AND column_name='history'
    ) THEN
        ALTER TABLE users ADD COLUMN history INTEGER DEFAULT 0;
    END IF;
END $$;
`

const addQuotedMessageIDSQL = `
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name='message_history' AND column_name='quoted_message_id'
    ) THEN
        ALTER TABLE message_history ADD COLUMN quoted_message_id TEXT;
    END IF;
END $$;
`

const addDataJsonSQL = `
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name='message_history' AND column_name='datajson'
    ) THEN
        ALTER TABLE message_history ADD COLUMN datajson TEXT;
    END IF;
END $$;
`

const addHmacKeySQL = `
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name='users' AND column_name='hmac_key'
    ) THEN
        ALTER TABLE users ADD COLUMN hmac_key BYTEA;
    END IF;
END $$;
`

const addWAContactsSQL = `
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name='wa_contacts') THEN
        CREATE TABLE wa_contacts (
            user_id TEXT NOT NULL,
            jid_phone TEXT NOT NULL,
            jid_lid TEXT,
            push_name TEXT,
            business_name TEXT,
            updated_at TIMESTAMP NOT NULL,
            PRIMARY KEY (user_id, jid_phone)
        );
        CREATE INDEX idx_wa_contacts_user_lid
            ON wa_contacts (user_id, jid_lid);
    END IF;
END $$;
`

/* ===================== SQLITE COMPAT ===================== */

// No SQLite não existe DO $$ / information_schema.
// Então: a migration 1 cria o schema "completo" (incluindo colunas/tabelas das migrações posteriores).
// Em bancos legados, as migrações seguintes continuam ajustando colunas/índices de forma idempotente.
const initialSchemaSQLiteSQL = `
CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  token TEXT NOT NULL,
  webhook TEXT NOT NULL DEFAULT '',
  jid TEXT NOT NULL DEFAULT '',
  qrcode TEXT NOT NULL DEFAULT '',
  connected INTEGER,
  expiration INTEGER,
  events TEXT NOT NULL DEFAULT '',
  proxy_url TEXT DEFAULT '',

  s3_enabled INTEGER DEFAULT 0,
  s3_endpoint TEXT DEFAULT '',
  s3_region TEXT DEFAULT '',
  s3_bucket TEXT DEFAULT '',
  s3_access_key TEXT DEFAULT '',
  s3_secret_key TEXT DEFAULT '',
  s3_path_style INTEGER DEFAULT 1,
  s3_public_url TEXT DEFAULT '',
  media_delivery TEXT DEFAULT 'base64',
  s3_retention_days INTEGER DEFAULT 30,

  history INTEGER DEFAULT 0,
  hmac_key BLOB
);

CREATE TABLE IF NOT EXISTS message_history (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id TEXT NOT NULL,
  chat_jid TEXT NOT NULL,
  sender_jid TEXT NOT NULL,
  message_id TEXT NOT NULL,
  timestamp DATETIME NOT NULL,
  message_type TEXT NOT NULL,
  text_content TEXT,
  media_link TEXT,
  quoted_message_id TEXT,
  datajson TEXT,
  UNIQUE(user_id, message_id)
);

CREATE INDEX IF NOT EXISTS idx_message_history_user_chat_timestamp
  ON message_history (user_id, chat_jid, timestamp DESC);

CREATE TABLE IF NOT EXISTS wa_contacts (
  user_id TEXT NOT NULL,
  jid_phone TEXT NOT NULL,
  jid_lid TEXT,
  push_name TEXT,
  business_name TEXT,
  updated_at DATETIME NOT NULL,
  PRIMARY KEY (user_id, jid_phone)
);

CREATE INDEX IF NOT EXISTS idx_wa_contacts_user_lid
  ON wa_contacts (user_id, jid_lid);
`

func upSQLFor(db *sqlx.DB, m Migration) string {
	if db.DriverName() == "sqlite" {
		return ""
	}
	return m.UpSQL
}

/* ===================== INIT ===================== */

func initializeSchema(db *sqlx.DB) error {
	if err := createMigrationsTable(db); err != nil {
		return err
	}

	applied, err := getAppliedMigrations(db)
	if err != nil {
		return err
	}

	for _, m := range migrations {
		if _, ok := applied[m.ID]; !ok {
			if err := applyMigration(db, m); err != nil {
				return fmt.Errorf("migration %d failed: %w", m.ID, err)
			}
		}
	}
	return nil
}

/* ===================== HELPERS ===================== */

func createMigrationsTable(db *sqlx.DB) error {
	switch db.DriverName() {
	case "postgres":
		_, err := db.Exec(`
			CREATE TABLE IF NOT EXISTS migrations (
				id INTEGER PRIMARY KEY,
				name TEXT NOT NULL,
				applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)`)
		return err
	case "sqlite":
		_, err := db.Exec(`
			CREATE TABLE IF NOT EXISTS migrations (
				id INTEGER PRIMARY KEY,
				name TEXT NOT NULL,
				applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
			)`)
		return err
	default:
		return fmt.Errorf("unsupported db")
	}
}

func getAppliedMigrations(db *sqlx.DB) (map[int]struct{}, error) {
	rows := []struct{ ID int }{}
	err := db.Select(&rows, `SELECT id FROM migrations`)
	if err != nil {
		return nil, err
	}
	out := map[int]struct{}{}
	for _, r := range rows {
		out[r.ID] = struct{}{}
	}
	return out, nil
}

func applyMigration(db *sqlx.DB, m Migration) error {
	tx, err := db.Beginx()
	if err != nil {
		return err
	}

	if db.DriverName() == "sqlite" {
		err = applySQLiteMigration(tx, m)
	} else {
		sql := strings.TrimSpace(upSQLFor(db, m))
		if sql != "" {
			_, err = tx.Exec(sql)
		}
	}
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	if db.DriverName() == "sqlite" {
		_, err = tx.Exec(`INSERT INTO migrations (id, name) VALUES (?, ?)`, m.ID, m.Name)
	} else {
		_, err = tx.Exec(`INSERT INTO migrations (id, name) VALUES ($1, $2)`, m.ID, m.Name)
	}

	if err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}

/* ===================== UTIL ===================== */

func GenerateRandomID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func applySQLiteMigration(tx *sqlx.Tx, m Migration) error {
	switch m.ID {
	case 1:
		if _, err := tx.Exec(initialSchemaSQLiteSQL); err != nil {
			return err
		}
	case 2:
		return ensureSQLiteColumn(tx, "users", "proxy_url", "TEXT DEFAULT ''")
	case 3:
		return migrateSQLiteUsersIDToString(tx)
	case 4:
		if err := ensureSQLiteColumn(tx, "users", "s3_enabled", "INTEGER DEFAULT 0"); err != nil {
			return err
		}
		if err := ensureSQLiteColumn(tx, "users", "s3_endpoint", "TEXT DEFAULT ''"); err != nil {
			return err
		}
		if err := ensureSQLiteColumn(tx, "users", "s3_region", "TEXT DEFAULT ''"); err != nil {
			return err
		}
		if err := ensureSQLiteColumn(tx, "users", "s3_bucket", "TEXT DEFAULT ''"); err != nil {
			return err
		}
		if err := ensureSQLiteColumn(tx, "users", "s3_access_key", "TEXT DEFAULT ''"); err != nil {
			return err
		}
		if err := ensureSQLiteColumn(tx, "users", "s3_secret_key", "TEXT DEFAULT ''"); err != nil {
			return err
		}
		if err := ensureSQLiteColumn(tx, "users", "s3_path_style", "INTEGER DEFAULT 1"); err != nil {
			return err
		}
		if err := ensureSQLiteColumn(tx, "users", "s3_public_url", "TEXT DEFAULT ''"); err != nil {
			return err
		}
		if err := ensureSQLiteColumn(tx, "users", "media_delivery", "TEXT DEFAULT 'base64'"); err != nil {
			return err
		}
		return ensureSQLiteColumn(tx, "users", "s3_retention_days", "INTEGER DEFAULT 30")
	case 5:
		if err := ensureSQLiteMessageHistoryTable(tx); err != nil {
			return err
		}
		return ensureSQLiteColumn(tx, "users", "history", "INTEGER DEFAULT 0")
	case 6:
		return ensureSQLiteColumn(tx, "message_history", "quoted_message_id", "TEXT")
	case 7:
		return ensureSQLiteColumn(tx, "users", "hmac_key", "BLOB")
	case 8:
		return ensureSQLiteColumn(tx, "message_history", "datajson", "TEXT")
	case 9:
		return ensureSQLiteWAContactsTable(tx)
	default:
		return nil
	}
	return nil
}

func ensureSQLiteTable(tx *sqlx.Tx, tableName, createSQL string) error {
	exists, err := sqliteTableExists(tx, tableName)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err = tx.Exec(createSQL)
	return err
}

func ensureSQLiteColumn(tx *sqlx.Tx, tableName, columnName, columnDef string) error {
	exists, err := sqliteColumnExists(tx, tableName, columnName)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err = tx.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", tableName, columnName, columnDef))
	return err
}

func ensureSQLiteIndex(tx *sqlx.Tx, indexName, createSQL string) error {
	exists, err := sqliteIndexExists(tx, indexName)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err = tx.Exec(createSQL)
	return err
}

func sqliteTableExists(tx *sqlx.Tx, tableName string) (bool, error) {
	var count int
	err := tx.Get(&count, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, tableName)
	return count > 0, err
}

func sqliteIndexExists(tx *sqlx.Tx, indexName string) (bool, error) {
	var count int
	err := tx.Get(&count, `SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?`, indexName)
	return count > 0, err
}

func sqliteColumnExists(tx *sqlx.Tx, tableName, columnName string) (bool, error) {
	rows, err := tx.Queryx(fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid       int
			name      string
			typ       string
			notnull   int
			dfltValue any
			pk        int
		)
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			return false, err
		}
		if name == columnName {
			return true, nil
		}
	}
	return false, rows.Err()
}

func sqliteColumnType(tx *sqlx.Tx, tableName, columnName string) (string, error) {
	rows, err := tx.Queryx(fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return "", err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid       int
			name      string
			typ       string
			notnull   int
			dfltValue any
			pk        int
		)
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			return "", err
		}
		if name == columnName {
			return strings.ToUpper(strings.TrimSpace(typ)), nil
		}
	}
	return "", rows.Err()
}

func ensureSQLiteMessageHistoryTable(tx *sqlx.Tx) error {
	if err := ensureSQLiteTable(tx, "message_history", `
		CREATE TABLE message_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			chat_jid TEXT NOT NULL,
			sender_jid TEXT NOT NULL,
			message_id TEXT NOT NULL,
			timestamp DATETIME NOT NULL,
			message_type TEXT NOT NULL,
			text_content TEXT,
			media_link TEXT,
			quoted_message_id TEXT,
			datajson TEXT
		)`); err != nil {
		return err
	}

	if err := ensureSQLiteColumn(tx, "message_history", "quoted_message_id", "TEXT"); err != nil {
		return err
	}
	if err := ensureSQLiteColumn(tx, "message_history", "datajson", "TEXT"); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		DELETE FROM message_history
		WHERE id NOT IN (
			SELECT MIN(id)
			FROM message_history
			GROUP BY user_id, message_id
		)`); err != nil {
		return err
	}
	if err := ensureSQLiteIndex(tx, "idx_message_history_user_chat_timestamp",
		`CREATE INDEX idx_message_history_user_chat_timestamp ON message_history (user_id, chat_jid, timestamp DESC)`); err != nil {
		return err
	}
	return ensureSQLiteIndex(tx, "idx_message_history_user_message_unique",
		`CREATE UNIQUE INDEX idx_message_history_user_message_unique ON message_history (user_id, message_id)`)
}

func ensureSQLiteWAContactsTable(tx *sqlx.Tx) error {
	if err := ensureSQLiteTable(tx, "wa_contacts", `
		CREATE TABLE wa_contacts (
			user_id TEXT NOT NULL,
			jid_phone TEXT NOT NULL,
			jid_lid TEXT,
			push_name TEXT,
			business_name TEXT,
			updated_at DATETIME NOT NULL
		)`); err != nil {
		return err
	}

	if err := ensureSQLiteColumn(tx, "wa_contacts", "jid_lid", "TEXT"); err != nil {
		return err
	}
	if err := ensureSQLiteColumn(tx, "wa_contacts", "push_name", "TEXT"); err != nil {
		return err
	}
	if err := ensureSQLiteColumn(tx, "wa_contacts", "business_name", "TEXT"); err != nil {
		return err
	}
	if err := ensureSQLiteColumn(tx, "wa_contacts", "updated_at", "DATETIME"); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		DELETE FROM wa_contacts
		WHERE rowid NOT IN (
			SELECT MIN(rowid)
			FROM wa_contacts
			GROUP BY user_id, jid_phone
		)`); err != nil {
		return err
	}
	if err := ensureSQLiteIndex(tx, "idx_wa_contacts_user_lid",
		`CREATE INDEX idx_wa_contacts_user_lid ON wa_contacts (user_id, jid_lid)`); err != nil {
		return err
	}
	return ensureSQLiteIndex(tx, "idx_wa_contacts_user_phone_unique",
		`CREATE UNIQUE INDEX idx_wa_contacts_user_phone_unique ON wa_contacts (user_id, jid_phone)`)
}

func migrateSQLiteUsersIDToString(tx *sqlx.Tx) error {
	usersExists, err := sqliteTableExists(tx, "users")
	if err != nil || !usersExists {
		return err
	}

	idType, err := sqliteColumnType(tx, "users", "id")
	if err != nil {
		return err
	}
	if idType != "INTEGER" {
		return nil
	}

	if _, err := tx.Exec(`DROP TABLE IF EXISTS users_new`); err != nil {
		return err
	}
	if _, err := tx.Exec(`
		CREATE TABLE users_new (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			token TEXT NOT NULL,
			webhook TEXT NOT NULL DEFAULT '',
			jid TEXT NOT NULL DEFAULT '',
			qrcode TEXT NOT NULL DEFAULT '',
			connected INTEGER,
			expiration INTEGER,
			events TEXT NOT NULL DEFAULT '',
			proxy_url TEXT DEFAULT '',
			s3_enabled INTEGER DEFAULT 0,
			s3_endpoint TEXT DEFAULT '',
			s3_region TEXT DEFAULT '',
			s3_bucket TEXT DEFAULT '',
			s3_access_key TEXT DEFAULT '',
			s3_secret_key TEXT DEFAULT '',
			s3_path_style INTEGER DEFAULT 1,
			s3_public_url TEXT DEFAULT '',
			media_delivery TEXT DEFAULT 'base64',
			s3_retention_days INTEGER DEFAULT 30,
			history INTEGER DEFAULT 0,
			hmac_key BLOB
		)`); err != nil {
		return err
	}

	columns := map[string]string{
		"name":              "''",
		"token":             "''",
		"webhook":           "''",
		"jid":               "''",
		"qrcode":            "''",
		"connected":         "NULL",
		"expiration":        "NULL",
		"events":            "''",
		"proxy_url":         "''",
		"s3_enabled":        "0",
		"s3_endpoint":       "''",
		"s3_region":         "''",
		"s3_bucket":         "''",
		"s3_access_key":     "''",
		"s3_secret_key":     "''",
		"s3_path_style":     "1",
		"s3_public_url":     "''",
		"media_delivery":    "'base64'",
		"s3_retention_days": "30",
		"history":           "0",
		"hmac_key":          "NULL",
	}

	selectExprs := []string{"hex(randomblob(16))"}
	insertCols := []string{"id"}
	for _, col := range []string{
		"name", "token", "webhook", "jid", "qrcode", "connected", "expiration", "events",
		"proxy_url", "s3_enabled", "s3_endpoint", "s3_region", "s3_bucket", "s3_access_key",
		"s3_secret_key", "s3_path_style", "s3_public_url", "media_delivery", "s3_retention_days",
		"history", "hmac_key",
	} {
		exists, err := sqliteColumnExists(tx, "users", col)
		if err != nil {
			return err
		}
		insertCols = append(insertCols, col)
		if exists {
			selectExprs = append(selectExprs, col)
		} else {
			selectExprs = append(selectExprs, columns[col])
		}
	}

	insertSQL := fmt.Sprintf(`
		INSERT INTO users_new (%s)
		SELECT %s FROM users`,
		strings.Join(insertCols, ", "),
		strings.Join(selectExprs, ", "),
	)
	if _, err := tx.Exec(insertSQL); err != nil {
		return err
	}
	if _, err := tx.Exec(`DROP TABLE users`); err != nil {
		return err
	}
	_, err = tx.Exec(`ALTER TABLE users_new RENAME TO users`)
	return err
}
