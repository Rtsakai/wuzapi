package main

import (
	"context"
	"strings"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/singleflight"
)

// Cache de "avatar indisponível" (privacidade / sem foto)
// Evita insistir e evita spam de log
var avatarDenyCache = cache.New(12*time.Hour, 30*time.Minute)

// Colapsa chamadas concorrentes para o mesmo user+jid
var avatarSF singleflight.Group

func isHiddenAvatarErr(err error) bool {
	if err == nil {
		return false
	}

	s := strings.ToLower(err.Error())

	// Privacidade / bloqueio
	if strings.Contains(s, "hidden their profile picture") ||
		(strings.Contains(s, "profile picture") && strings.Contains(s, "hidden")) ||
		strings.Contains(s, "privacy") ||
		strings.Contains(s, "not authorized") ||
		strings.Contains(s, "forbidden") ||
		strings.Contains(s, "403") {
		return true
	}

	// Sem foto
	if strings.Contains(s, "does not have a profile picture") ||
		strings.Contains(s, "no profile picture") ||
		(strings.Contains(s, "profile picture") && strings.Contains(s, "does not have")) ||
		(strings.Contains(s, "profile picture") && strings.Contains(s, "not have")) {
		return true
	}

	return false
}

// Aplica cooldown quando sabemos que não há avatar
func avatarCooldown(denyKey string) {
	avatarDenyCache.SetDefault(denyKey, true)
	log.Debug().
		Str("denyKey", denyKey).
		Msg("avatar not available; cooldown applied")
}

// Contexto padrão para requests de avatar
func avatarCtx() (context.Context, func()) {
	return context.WithTimeout(context.Background(), 12*time.Second)
}
