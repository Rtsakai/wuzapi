package main

import (
	"context"
	"database/sql"
	"strings"
)

func ResolveLID(ctx context.Context, db *sql.DB, jid string) (string, bool) {
	if jid == "" {
		return "", false
	}

	// já é real → não trocou
	if strings.HasSuffix(jid, "@s.whatsapp.net") || strings.HasSuffix(jid, "@g.us") {
		return jid, false
	}

	if !strings.HasSuffix(jid, "@lid") {
		return jid, false
	}

	lid := strings.TrimSpace(strings.TrimSuffix(jid, "@lid"))
	if lid == "" {
		return jid, false
	}

	var pn string
	err := db.QueryRowContext(ctx,
		`select pn from whatsmeow_lid_map where lid = $1`,
		lid,
	).Scan(&pn)
	if err != nil || pn == "" {
		return jid, false
	}

	return pn + "@s.whatsapp.net", true // aqui sim: trocou
}
