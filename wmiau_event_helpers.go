package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

type cachedMediaConfig struct {
	Enabled       string `db:"s3_enabled"`
	MediaDelivery string `db:"media_delivery"`
}

func (mycli *MyClient) loadMediaConfig(userID string) cachedMediaConfig {
	var cfg cachedMediaConfig
	myuserinfo, found := userinfocache.Get(mycli.token)
	if !found {
		err := mycli.db.Get(&cfg, "SELECT CASE WHEN s3_enabled = 1 THEN 'true' ELSE 'false' END AS s3_enabled, media_delivery FROM users WHERE id = $1", userID)
		if err != nil {
			log.Error().Err(err).Msg("onMessage Failed to get S3 config from DB as it was not on cache")
			cfg.Enabled = "false"
			cfg.MediaDelivery = "base64"
		}
		return cfg
	}

	cfg.Enabled = myuserinfo.(Values).Get("S3Enabled")
	cfg.MediaDelivery = myuserinfo.(Values).Get("MediaDelivery")
	return cfg
}

func normalizedContactJID(info *types.MessageInfo) string {
	contactJID := info.Sender.String()
	if info.IsGroup {
		contactJID = info.Chat.String()
	}
	return contactJID
}

func (mycli *MyClient) processIncomingMedia(postmap map[string]interface{}, info *types.MessageInfo, data []byte, mimeType, defaultExt, logLabel string, extra map[string]interface{}, mediaConfig cachedMediaConfig) error {
	return mediaService.ProcessIncomingMedia(context.Background(), postmap, data, IncomingMediaOptions{
		UserID:        mycli.userID,
		ContactJID:    normalizedContactJID(info),
		MessageID:     info.ID,
		MimeType:      mimeType,
		DefaultExt:    defaultExt,
		IsIncoming:    !info.IsFromMe,
		MediaDelivery: mediaConfig.MediaDelivery,
		S3Enabled:     mediaConfig.Enabled == "true",
		LogLabel:      logLabel,
		ExtraPostmap:  extra,
	})
}

func (mycli *MyClient) processIncomingDocument(postmap map[string]interface{}, info *types.MessageInfo, document *waE2E.DocumentMessage, mediaConfig cachedMediaConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	data, err := mediaService.DownloadDocument(ctx, mycli.WAClient, document)
	if err != nil {
		return fmt.Errorf("download document: %w", err)
	}

	extra := map[string]interface{}{}
	return mycli.processIncomingMedia(postmap, info, data, document.GetMimetype(), ".bin", "Document", extra, mediaConfig)
}

func (mycli *MyClient) processIncomingSticker(postmap map[string]interface{}, info *types.MessageInfo, sticker *waE2E.StickerMessage, mediaConfig cachedMediaConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	log.Debug().
		Str("sticker_url", sticker.GetURL()).
		Str("sticker_direct_path", sticker.GetDirectPath()).
		Bool("directpath_has_auth", strings.Contains(sticker.GetDirectPath(), "auth=")).
		Int("media_key_len", len(sticker.GetMediaKey())).
		Msg("Sticker download debug")

	data, err := mediaService.DownloadSticker(ctx, mycli.WAClient, sticker)
	if err != nil {
		return fmt.Errorf("download sticker: %w", err)
	}

	return mycli.processIncomingMedia(postmap, info, data, sticker.GetMimetype(), ".webp", "Sticker", map[string]interface{}{
		"isSticker":       true,
		"stickerAnimated": sticker.GetIsAnimated(),
	}, mediaConfig)
}

func (mycli *MyClient) saveIncomingEventHistory(evt *events.Message, postmap map[string]interface{}) {
	var historyLimit int
	userinfo, found := userinfocache.Get(mycli.token)
	if found {
		historyStr := userinfo.(Values).Get("History")
		historyLimit, _ = strconv.Atoi(historyStr)
	} else {
		log.Warn().Str("userID", mycli.userID).Msg("User info not found in cache, skipping history")
		historyLimit = 0
	}

	if historyLimit <= 0 {
		return
	}

	messageType := "text"
	textContent := ""
	mediaLink := ""
	caption := ""
	replyToMessageID := ""

	if protocolMsg := evt.Message.GetProtocolMessage(); protocolMsg != nil && protocolMsg.GetType() == 0 {
		messageType = "delete"
		if protocolMsg.GetKey() != nil {
			textContent = protocolMsg.GetKey().GetID()
		}
		log.Info().Str("deletedMessageID", textContent).Str("messageID", evt.Info.ID).Msg("Delete message detected")
	} else if reaction := evt.Message.GetReactionMessage(); reaction != nil {
		messageType = "reaction"
		replyToMessageID = reaction.GetKey().GetID()
		textContent = reaction.GetText()
	} else if img := evt.Message.GetImageMessage(); img != nil {
		messageType = "image"
		caption = img.GetCaption()
	} else if video := evt.Message.GetVideoMessage(); video != nil {
		messageType = "video"
		caption = video.GetCaption()
	} else if audio := evt.Message.GetAudioMessage(); audio != nil {
		messageType = "audio"
	} else if doc := evt.Message.GetDocumentMessage(); doc != nil {
		messageType = "document"
		caption = doc.GetCaption()
	} else if sticker := evt.Message.GetStickerMessage(); sticker != nil {
		messageType = "sticker"
	} else if contact := evt.Message.GetContactMessage(); contact != nil {
		messageType = "contact"
		textContent = contact.GetDisplayName()
	} else if location := evt.Message.GetLocationMessage(); location != nil {
		messageType = "location"
		textContent = location.GetName()
	}

	if messageType != "reaction" && messageType != "delete" {
		if conv := evt.Message.GetConversation(); conv != "" {
			textContent = conv
		} else if ext := evt.Message.GetExtendedTextMessage(); ext != nil {
			textContent = ext.GetText()
			if contextInfo := ext.GetContextInfo(); contextInfo != nil && contextInfo.GetStanzaID() != "" {
				replyToMessageID = contextInfo.GetStanzaID()
			}
		} else {
			textContent = caption
		}

		if textContent == "" {
			switch messageType {
			case "image":
				textContent = ":image:"
			case "video":
				textContent = ":video:"
			case "audio":
				textContent = ":audio:"
			case "document":
				textContent = ":document:"
			case "sticker":
				textContent = ":sticker:"
			case "contact":
				textContent = ":contact:"
			case "location":
				textContent = ":location:"
			}
		}
	}

	if s3Data, ok := postmap["s3"].(map[string]interface{}); ok {
		if url, ok := s3Data["url"].(string); ok {
			mediaLink = url
		}
	}

	if textContent == "" && mediaLink == "" && messageType == "text" {
		log.Debug().Str("messageType", messageType).Str("messageID", evt.Info.ID).Msg("Skipping empty message from history")
		return
	}

	evtJSON, err := json.Marshal(evt)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal event to JSON")
		evtJSON = []byte("{}")
	}

	err = mycli.s.saveMessageToHistory(
		mycli.userID,
		evt.Info.Chat.String(),
		evt.Info.Sender.String(),
		evt.Info.ID,
		messageType,
		textContent,
		mediaLink,
		replyToMessageID,
		string(evtJSON),
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to save message to history")
		return
	}

	if err := mycli.s.trimMessageHistory(mycli.userID, evt.Info.Chat.String(), historyLimit); err != nil {
		log.Error().Err(err).Msg("Failed to trim message history")
	}
}
