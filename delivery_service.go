package main

import (
	"encoding/base64"
	"encoding/json"

	"github.com/rs/zerolog/log"
)

type DeliveryService struct{}

func NewDeliveryService() *DeliveryService {
	return &DeliveryService{}
}

func (d *DeliveryService) userInstanceName(token string) string {
	if userinfo, found := userinfocache.Get(token); found {
		return userinfo.(Values).Get("Name")
	}
	return ""
}

func (d *DeliveryService) SendGlobalWebhook(jsonData []byte, token string, userID string) {
	if *globalWebhook == "" {
		return
	}

	globalData := map[string]string{
		"jsonData":     string(jsonData),
		"userID":       userID,
		"instanceName": d.userInstanceName(token),
	}
	log.Info().Str("url", *globalWebhook).Msg("Calling global webhook")
	callHookWithHmac(*globalWebhook, globalData, userID, globalHMACKeyEncrypted)
}

func (d *DeliveryService) SendUserWebhook(webhookurl string, path string, jsonData []byte, userID string, token string, encryptedHmacKey []byte) {
	data := map[string]string{
		"jsonData":     string(jsonData),
		"userID":       userID,
		"instanceName": d.userInstanceName(token),
	}

	log.Debug().Interface("webhookData", data).Msg("Data being sent to webhook")
	if webhookurl == "" {
		log.Warn().Str("userid", userID).Msg("No webhook set for user")
		return
	}

	log.Info().Str("url", webhookurl).Msg("Calling user webhook")
	if path == "" {
		go callHookWithHmac(webhookurl, data, userID, encryptedHmacKey)
		return
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- callHookFileWithHmac(webhookurl, data, userID, path, encryptedHmacKey)
	}()

	if err := <-errChan; err != nil {
		log.Error().Err(err).Msg("Error calling hook file")
	}
}

func (d *DeliveryService) DispatchEvent(mycli *MyClient, postmap map[string]interface{}, path string) {
	webhookurl := getUserWebhookUrl(mycli.token)
	subscribedEvents, err := updateAndGetUserSubscriptions(mycli)
	if err != nil {
		return
	}

	eventType, ok := postmap["type"].(string)
	if !ok {
		log.Error().Msg("Event type is not a string in postmap")
		return
	}

	log.Debug().
		Str("userID", mycli.userID).
		Str("eventType", eventType).
		Strs("subscribedEvents", subscribedEvents).
		Msg("Checking event subscription")

	if !checkIfSubscribedToEvent(subscribedEvents, eventType, mycli.userID) {
		return
	}

	if mycli.s != nil && mycli.s.mode == Stdio {
		mycli.s.SendNotification(eventType, postmap)
		return
	}

	jsonData, err := json.Marshal(postmap)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal postmap to JSON")
		return
	}

	var encryptedHmacKey []byte
	if userinfo, found := userinfocache.Get(mycli.token); found {
		encryptedB64 := userinfo.(Values).Get("HmacKeyEncrypted")
		if encryptedB64 != "" {
			encryptedHmacKey, err = base64.StdEncoding.DecodeString(encryptedB64)
			if err != nil {
				log.Error().Err(err).Msg("Failed to decode HMAC key from cache")
			}
		}
	}

	d.SendUserWebhook(webhookurl, path, jsonData, mycli.userID, mycli.token, encryptedHmacKey)
	go d.SendGlobalWebhook(jsonData, mycli.token, mycli.userID)
	go sendToGlobalRabbit(jsonData, mycli.token, mycli.userID)
}

var deliveryService = NewDeliveryService()
