package main

import (
	"context"
	"errors"

	"go.mau.fi/whatsmeow"
)

type WhatsMeowAdapter struct{}

func NewWhatsMeowAdapter() *WhatsMeowAdapter {
	return &WhatsMeowAdapter{}
}

func (a *WhatsMeowAdapter) Client(userID string) *whatsmeow.Client {
	return clientManager.GetWhatsmeowClient(userID)
}

func (a *WhatsMeowAdapter) RequireClient(userID string) (*whatsmeow.Client, error) {
	client := a.Client(userID)
	if client == nil {
		return nil, errors.New("no session")
	}
	return client, nil
}

func (a *WhatsMeowAdapter) Download(ctx context.Context, userID string, media whatsmeow.DownloadableMessage) ([]byte, error) {
	client, err := a.RequireClient(userID)
	if err != nil {
		return nil, err
	}
	return mediaService.Download(ctx, client, media)
}

var whatsmeowAdapter = NewWhatsMeowAdapter()
