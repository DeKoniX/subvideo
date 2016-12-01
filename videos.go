package main

import (
	"log"
	"net/http"
	"time"

	"google.golang.org/api/googleapi/transport"
	youtube "google.golang.org/api/youtube/v3"
)

type SubVideo struct {
	TypeSub     string
	Title       string
	Channel     string
	ChannelID   string
	Game        string
	Description string
	URL         string
	ThumbURL    string
	Length      int
	Date        time.Time
}

type ChannelOnline struct {
	Title     string
	Channel   string
	ChannelID string
	URL       string
	Game      string
	ThumbURL  string
}

type ClientVideo struct {
	client   *http.Client
	dataBase DB
	ytClient *youtube.Service
	twClient TW
}

func InitClientVideo(twClientID, twClientSecret, ytDeveloperKey string) (clientVideo ClientVideo) {
	clientVideo.client = &http.Client{
		Transport: &transport.APIKey{Key: ytDeveloperKey},
	}
	clientVideo.twClient = TWInit(clientVideo.client, twClientID, twClientSecret)
	clientVideo.ytClient, _ = youtube.New(clientVideo.client)

	return clientVideo
}

func (clientVideo ClientVideo) SortVideo(user User, n int, channelID string) (subVideos []SubVideo, err error) {
	subVideos, err = clientVideo.dataBase.SelectVideo(user.ID, n, channelID)
	if err != nil {
		log.Fatal(err)
		return subVideos, err
	}

	return subVideos, nil
}

func (clientVideo ClientVideo) TWGetVideo(user User) (err error) {
	videos := clientVideo.twClient.GetVideos(user.TWOAuth)
	for _, video := range videos {
		err = clientVideo.dataBase.InsertVideo(
			user.ID,
			video.TypeSub,
			video.Title,
			video.Channel,
			video.ChannelID,
			video.Game,
			video.Description,
			video.URL,
			video.ThumbURL,
			video.Length,
			video.Date.UTC(),
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (clientVideo ClientVideo) YTGetVideo(user User) (err error) {
	if user.YTChannelID == "" {
		return nil
	}

	call := clientVideo.ytClient.Subscriptions.List("snippet").
		ChannelId(user.YTChannelID).
		MaxResults(50)
	response, err := call.Do()
	if err != nil {
		return err
	}

	for _, item := range response.Items {
		channelID := item.Snippet.ResourceId.ChannelId

		callVideo := clientVideo.ytClient.Search.List("snippet").
			Q("").
			ChannelId(channelID).
			MaxResults(5).
			Order("date").
			Type("video")

		responseVideo, err := callVideo.Do()
		if err != nil {
			return err
		}

		for _, video := range responseVideo.Items {
			ytTime, err := time.Parse(time.RFC3339, video.Snippet.PublishedAt)
			if err != nil {
				return err
			}

			err = clientVideo.dataBase.InsertVideo(
				user.ID,
				"youtube",
				video.Snippet.Title,
				video.Snippet.ChannelTitle,
				video.Snippet.ChannelId,
				"",
				video.Snippet.Description,
				"https://www.youtube.com/watch?v="+video.Id.VideoId,
				video.Snippet.Thumbnails.High.Url,
				1000,
				ytTime.UTC(),
			)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
