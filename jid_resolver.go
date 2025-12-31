package main

import (
	"strings"

	"github.com/jmoiron/sqlx"
)

func resolveLIDToPhoneJIDNoCtx(db *sqlx.DB, jid string) string {
	if !strings.HasSuffix(jid, "@lid") {
		return jid
	}

	lid := strings.TrimSuffix(jid, "@lid")

	var pn string
	q := `SELECT pn FROM whatsmeow_lid_map WHERE lid = $1 LIMIT 1`
	if db.DriverName() == "sqlite" {
		q = `SELECT pn FROM whatsmeow_lid_map WHERE lid = ? LIMIT 1`
	}

	err := db.QueryRow(q, lid).Scan(&pn)
	if err != nil || pn == "" {
		return jid // fallback: mantém @lid
	}

	return pn + "@s.whatsapp.net"
}
