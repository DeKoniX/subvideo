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
	client     *http.Client
	twClientID string
	TimeZone   *time.Location
	DataBase   DB
}

func InitClientVideo(twClientID, ytDeveloperKey string) (clientVideo ClientVideo) {
	clientVideo.client = &http.Client{
		Transport: &transport.APIKey{Key: ytDeveloperKey},
	}
	clientVideo.twClientID = twClientID
	clientVideo.TimeZone, _ = time.LoadLocation("UTC")

	return clientVideo
}

func (clientVideo ClientVideo) SortVideo(user User, n int, channelID string) (subVideos []SubVideo, err error) {
	subVideos, err = clientVideo.DataBase.SelectVideo(user.ID, n, channelID)
	if err != nil {
		log.Fatal(err)
		return subVideos, err
	}

	return subVideos, nil
}

func (clientVideo ClientVideo) TWGetVideo(user User) (err error) {
	videos := tw.GetVideos(user.TWOAuth)
	for _, video := range videos {
		err = clientVideo.DataBase.InsertVideo(
			user.ID,
			video.TypeSub,
			video.Title,
			video.Channel,
			video.ChannelID,
			video.Game,
			video.Description,
			video.URL,
			video.ThumbURL,
			video.Date,
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

	service, _ := youtube.New(clientVideo.client)
	call := service.Subscriptions.List("snippet").
		ChannelId(user.YTChannelID).
		MaxResults(50)
	response, err := call.Do()
	if err != nil {
		return err
	}

	for _, item := range response.Items {
		channelID := item.Snippet.ResourceId.ChannelId

		callVideo := service.Search.List("snippet").
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

			err = clientVideo.DataBase.InsertVideo(
				user.ID,
				"youtube",
				video.Snippet.Title,
				video.Snippet.ChannelTitle,
				video.Snippet.ChannelId,
				"",
				video.Snippet.Description,
				"https://www.youtube.com/watch?v="+video.Id.VideoId,
				video.Snippet.Thumbnails.High.Url,
				ytTime.In(clientVideo.TimeZone),
			)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
