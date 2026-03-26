package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

func newBenchmarkServer(b *testing.B) *server {
	b.Helper()

	db, err := sqlx.Open("sqlite", ":memory:")
	if err != nil {
		b.Fatalf("failed to create benchmark database: %v", err)
	}
	b.Cleanup(func() { _ = db.Close() })

	if err := initializeSchema(db); err != nil {
		b.Fatalf("failed to initialize schema: %v", err)
	}

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS whatsmeow_lid_map (lid TEXT, pn TEXT)`); err != nil {
		b.Fatalf("failed to create lid map table: %v", err)
	}

	return &server{db: db}
}

func BenchmarkInitializeSchemaSQLiteMemory(b *testing.B) {
	for i := 0; i < b.N; i++ {
		db, err := sqlx.Open("sqlite", ":memory:")
		if err != nil {
			b.Fatalf("failed to open sqlite database: %v", err)
		}
		if err := initializeSchema(db); err != nil {
			_ = db.Close()
			b.Fatalf("failed to initialize schema: %v", err)
		}
		_ = db.Close()
	}
}

func BenchmarkSaveMessageToHistorySQLite(b *testing.B) {
	s := newBenchmarkServer(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := s.saveMessageToHistory(
			"user-1",
			"5511999999999@s.whatsapp.net",
			"5511888888888@s.whatsapp.net",
			fmt.Sprintf("msg-%d", i),
			"text",
			"payload",
			"",
			"",
			`{"kind":"benchmark"}`,
		)
		if err != nil {
			b.Fatalf("failed to save message: %v", err)
		}
	}
}

func BenchmarkSanitizeWebhookBodyForLog(b *testing.B) {
	payload := map[string]interface{}{
		"type":      "Message",
		"mime_type": "image/jpeg",
		"base64":    strings.Repeat("a", 32*1024),
		"message":   "benchmark",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _, _, _, err := sanitizeWebhookBodyForLog(payload)
		if err != nil {
			b.Fatalf("failed to sanitize payload: %v", err)
		}
	}
}
