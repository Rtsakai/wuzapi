package main

import (
	"context"
	"fmt"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
)

type MediaService struct{}

type MediaDeliveryOptions struct {
	UserID        string
	ContactJID    string
	MessageID     string
	MimeType      string
	FileName      string
	IsIncoming    bool
	MediaDelivery string
	S3Enabled     bool
}

type IncomingMediaOptions struct {
	UserID        string
	ContactJID    string
	MessageID     string
	MimeType      string
	FallbackName  string
	DefaultExt    string
	IsIncoming    bool
	MediaDelivery string
	S3Enabled     bool
	LogLabel      string
	ExtraPostmap  map[string]interface{}
}

func NewMediaService() *MediaService {
	return &MediaService{}
}

func (m *MediaService) Download(ctx context.Context, cli *whatsmeow.Client, media whatsmeow.DownloadableMessage) ([]byte, error) {
	return downloadWithRetry(ctx, cli, media)
}

func (m *MediaService) DownloadDocument(ctx context.Context, cli *whatsmeow.Client, document *waE2E.DocumentMessage) ([]byte, error) {
	data, err := cli.Download(ctx, document)
	if err != nil && strings.Contains(err.Error(), "status code 403") {
		docCopy := *document
		if (docCopy.DirectPath == nil || docCopy.GetDirectPath() == "") && docCopy.GetURL() != "" {
			if u, perr := url.Parse(docCopy.GetURL()); perr == nil && u.Path != "" {
				docCopy.DirectPath = stringPtr(u.Path)
			}
		}
		docCopy.URL = nil
		data, err = cli.Download(ctx, &docCopy)
	}
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (m *MediaService) DownloadSticker(ctx context.Context, cli *whatsmeow.Client, sticker *waE2E.StickerMessage) ([]byte, error) {
	data, err := cli.Download(ctx, sticker)
	if err == nil {
		return data, nil
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "status code 403") && !strings.Contains(errStr, "403") {
		return nil, err
	}

	st2 := *sticker
	empty := ""
	st2.URL = &empty
	for i := 0; i < 2; i++ {
		time.Sleep(time.Duration(250*(i+1)) * time.Millisecond)
		data, err = cli.Download(ctx, &st2)
		if err == nil {
			return data, nil
		}
	}

	return nil, err
}

func (m *MediaService) TempDir(userID string) (string, error) {
	tmpDirectory := filepath.Join("/tmp", "user_"+userID)
	if err := os.MkdirAll(tmpDirectory, 0751); err != nil {
		return "", err
	}
	return tmpDirectory, nil
}

func (m *MediaService) ResolveExtension(mimeType, fallbackName, defaultExt string) string {
	exts, err := mime.ExtensionsByType(mimeType)
	if err == nil && len(exts) > 0 && exts[0] != "" {
		return exts[0]
	}
	if fallbackName != "" {
		if ext := filepath.Ext(fallbackName); ext != "" {
			return ext
		}
	}
	if defaultExt == "" {
		return ".bin"
	}
	return defaultExt
}

func (m *MediaService) WriteTempFile(userID, messageID, mimeType, fallbackName, defaultExt string, data []byte) (string, error) {
	tmpDirectory, err := m.TempDir(userID)
	if err != nil {
		return "", fmt.Errorf("could not create temporary directory: %w", err)
	}

	ext := m.ResolveExtension(mimeType, fallbackName, defaultExt)
	tmpPath := filepath.Join(tmpDirectory, messageID+ext)
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return "", err
	}

	return tmpPath, nil
}

func (m *MediaService) EnrichPostmapWithMedia(ctx context.Context, postmap map[string]interface{}, tmpPath string, data []byte, opts MediaDeliveryOptions) error {
	if opts.S3Enabled && (opts.MediaDelivery == "s3" || opts.MediaDelivery == "both") {
		s3Data, err := GetS3Manager().ProcessMediaForS3(
			ctx,
			opts.UserID,
			opts.ContactJID,
			opts.MessageID,
			data,
			opts.MimeType,
			filepath.Base(tmpPath),
			opts.IsIncoming,
		)
		if err != nil {
			return err
		}
		postmap["s3"] = s3Data
	}

	if opts.MediaDelivery == "base64" || opts.MediaDelivery == "both" {
		base64String, mimeType, err := fileToBase64(tmpPath)
		if err != nil {
			return err
		}
		postmap["base64"] = base64String
		postmap["mimeType"] = mimeType
		postmap["fileName"] = filepath.Base(tmpPath)
	}

	return nil
}

func (m *MediaService) CleanupTempFile(path string) {
	if err := os.Remove(path); err != nil {
		log.Error().Err(err).Str("path", path).Msg("Failed to delete temporary file")
	}
}

func (m *MediaService) ProcessIncomingMedia(ctx context.Context, postmap map[string]interface{}, data []byte, opts IncomingMediaOptions) error {
	tmpPath, err := m.WriteTempFile(opts.UserID, opts.MessageID, opts.MimeType, opts.FallbackName, opts.DefaultExt, data)
	if err != nil {
		return err
	}
	defer m.CleanupTempFile(tmpPath)

	err = m.EnrichPostmapWithMedia(ctx, postmap, tmpPath, data, MediaDeliveryOptions{
		UserID:        opts.UserID,
		ContactJID:    opts.ContactJID,
		MessageID:     opts.MessageID,
		MimeType:      opts.MimeType,
		FileName:      filepath.Base(tmpPath),
		IsIncoming:    opts.IsIncoming,
		MediaDelivery: opts.MediaDelivery,
		S3Enabled:     opts.S3Enabled,
	})
	if err != nil {
		return err
	}

	for k, v := range opts.ExtraPostmap {
		postmap[k] = v
	}

	if opts.LogLabel != "" {
		log.Info().Str("path", tmpPath).Msg(opts.LogLabel + " processed")
		log.Info().Str("path", tmpPath).Msg("Temporary file deleted")
	}

	return nil
}

var mediaService = NewMediaService()

func stringPtr(s string) *string {
	return &s
}
