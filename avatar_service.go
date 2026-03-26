package main

import (
	"errors"
	"fmt"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

type AvatarService struct{}

func NewAvatarService() *AvatarService {
	return &AvatarService{}
}

func (a *AvatarService) GetProfilePictureInfo(cli *whatsmeow.Client, userID string, jid types.JID, preview bool) (*types.ProfilePictureInfo, error) {
	if cli == nil {
		return nil, errors.New("no session")
	}

	denyKey := userID + ":" + jid.String()
	if _, found := avatarDenyCache.Get(denyKey); found {
		return nil, errors.New("no avatar found")
	}

	sfKey := "avatar:" + denyKey
	val, err, _ := avatarSF.Do(sfKey, func() (any, error) {
		ctx, cancel := avatarCtx()
		defer cancel()

		pic, err := cli.GetProfilePictureInfo(ctx, jid, &whatsmeow.GetProfilePictureParams{
			Preview:    preview,
			ExistingID: "",
		})
		if err != nil {
			if isHiddenAvatarErr(err) {
				avatarCooldown(denyKey)
				return nil, nil
			}
			return nil, fmt.Errorf("failed to get avatar: %w", err)
		}

		return pic, nil
	})
	if err != nil {
		if isHiddenAvatarErr(err) {
			avatarCooldown(denyKey)
			return nil, errors.New("no avatar found")
		}
		return nil, err
	}

	pic, _ := val.(*types.ProfilePictureInfo)
	if pic == nil {
		return nil, errors.New("no avatar found")
	}

	return pic, nil
}

var avatarService = NewAvatarService()
