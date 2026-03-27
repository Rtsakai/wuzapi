package main

import "github.com/jmoiron/sqlx"

type HistoryRepository struct {
	db *sqlx.DB
}

type ContactRepository struct {
	db *sqlx.DB
}

type UserConfigRepository struct {
	db *sqlx.DB
}

type UserS3Config struct {
	Enabled       bool   `db:"enabled" json:"enabled"`
	Endpoint      string `db:"endpoint" json:"endpoint"`
	Region        string `db:"region" json:"region"`
	Bucket        string `db:"bucket" json:"bucket"`
	AccessKey     string `db:"access_key" json:"access_key"`
	SecretKey     string `db:"secret_key" json:"secret_key"`
	PathStyle     bool   `db:"path_style" json:"path_style"`
	PublicURL     string `db:"public_url" json:"public_url"`
	MediaDelivery string `db:"media_delivery" json:"media_delivery"`
	RetentionDays int    `db:"retention_days" json:"retention_days"`
}

func NewHistoryRepository(db *sqlx.DB) *HistoryRepository {
	return &HistoryRepository{db: db}
}

func NewContactRepository(db *sqlx.DB) *ContactRepository {
	return &ContactRepository{db: db}
}

func NewUserConfigRepository(db *sqlx.DB) *UserConfigRepository {
	return &UserConfigRepository{db: db}
}

func (r *HistoryRepository) SaveMessage(s *server, userID, chatJID, senderJID, messageID, messageType, textContent, mediaLink, quotedMessageID, dataJSON string) error {
	return s.saveMessageToHistory(userID, chatJID, senderJID, messageID, messageType, textContent, mediaLink, quotedMessageID, dataJSON)
}

func (r *HistoryRepository) GetMessageByID(userID, messageID string) (HistoryMessage, error) {
	var msg HistoryMessage
	query := `
		SELECT id, user_id, chat_jid, sender_jid, message_id, timestamp, message_type, text_content, media_link, COALESCE(quoted_message_id, '') as quoted_message_id, COALESCE(datajson, '') as datajson
		FROM message_history
		WHERE user_id = $1 AND message_id = $2
		LIMIT 1`

	if r.db.DriverName() == "sqlite" {
		query = `
			SELECT id, user_id, chat_jid, sender_jid, message_id, timestamp, message_type, text_content, media_link, COALESCE(quoted_message_id, '') as quoted_message_id, COALESCE(datajson, '') as datajson
			FROM message_history
			WHERE user_id = ? AND message_id = ?
			LIMIT 1`
	}

	err := r.db.Get(&msg, query, userID, messageID)
	if err != nil {
		return HistoryMessage{}, err
	}

	msg.ChatJID = normalizeChatJID(r.db, msg.ChatJID)
	msg.SenderJID = normalizeChatJID(r.db, msg.SenderJID)
	return msg, nil
}

func (r *ContactRepository) UpsertContactName(userID, jidPhone, jidLid, pushName, businessName string) error {
	return upsertContactName(r.db, userID, jidPhone, jidLid, pushName, businessName)
}

func (r *UserConfigRepository) GetUserS3Config(userID string) (UserS3Config, error) {
	var config UserS3Config
	err := r.db.Get(&config, `
		SELECT
			s3_enabled as enabled,
			s3_endpoint as endpoint,
			s3_region as region,
			s3_bucket as bucket,
			s3_access_key as access_key,
			s3_secret_key as secret_key,
			s3_path_style as path_style,
			s3_public_url as public_url,
			media_delivery,
			s3_retention_days as retention_days
		FROM users WHERE id = $1`, userID)
	return config, err
}
