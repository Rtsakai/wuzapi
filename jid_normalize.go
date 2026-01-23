package main

import "github.com/jmoiron/sqlx"

// Normaliza JID (@lid → @s.whatsapp.net) antes de persistir ou responder
func normalizeChatJID(db *sqlx.DB, jid string) string {
	if jid == "" {
		return jid
	}
	return resolveLIDToPhoneJIDNoCtx(db, jid)
}
