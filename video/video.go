package video

import "github.com/DeKoniX/subvideo/models"

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
	return subVideos, nil
}

func (client *ClientVideo) TWGetVideo(user models.User) (err error) {
	videos := client.TWClient.GetVideos(user.TWOAuth)
	for _, video := range videos {
		video.UserID = user.Id
		video.Insert()
	}
	return nil
}

func (client *ClientVideo) YTGetVideo(user models.User) (err error) {
	videos, err := client.YTClient.GetVideos(user)
	if err != nil {
		return err
	}

	for _, video := range videos {
		video.UserID = user.Id
		video.Insert()
	}

	return nil
}
