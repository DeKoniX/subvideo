package video

import (
	"time"

	"github.com/DeKoniX/subvideo/models"
)

type ChannelOnline struct {
	Title     string
	Channel   string
	ChannelID string
	URL       string
	Game      string
	ThumbURL  string
}

type ClientVideo struct {
	YTClient *YT
	TWClient *TW
}

func Init(twClientID, twClientSecret, twRedirectURI, ytClientID, ytClientSecret, ytRedirectURI string) (client *ClientVideo) {
	return &ClientVideo{
		TWClient: TWInit(twClientID, twClientSecret, twRedirectURI),
		YTClient: YTInit(ytClientID, ytClientSecret, ytRedirectURI),
	}
}

func (client *ClientVideo) SortVideo(user models.User, n int, channelID string, page int) (subVideos []models.Subvideo, err error) {
	subVideos, err = models.SelectVideo(int(user.Id), n, channelID, page)
	if err != nil {
		return subVideos, err
	}
	for _, video := range subVideos {
		var timezone *time.Location
		if user.TimeZone == "" {
			timezone, _ = time.LoadLocation("UTC")
		} else {
			timezone, _ = time.LoadLocation(user.TimeZone)
		}
		video.Date = video.Date.In(timezone)
	}
	return subVideos, nil
}

func (client *ClientVideo) TWGetVideo(user models.User) (err error) {
	if user.TWOAuth != "" || user.TWChannelID != "" {
		videos := client.TWClient.GetVideos(user.TWOAuth)
		for _, video := range videos {
			video.UserID = user.Id
			video.Insert()
		}
	} else {
		user.TWChannelID = ""
		user.TWOAuth = ""
		user.Insert()
	}
	return nil
}

func (client *ClientVideo) YTGetVideo(user models.User) (err error) {
	if user.YTOAuth != "" || user.YTChannelID != "" {
		videos, err := client.YTClient.GetVideos(user)
		if err != nil {
			return err
		}

		for _, video := range videos {
			video.UserID = user.Id
			video.Insert()
		}
	} else {
		user.YTChannelID = ""
		user.YTOAuth = ""
		user.YTRefreshToken = ""
		user.Insert()
	}

	return nil
}
