package main

import (
	"net/http"
	"strings"
	"time"

	"github.com/DeKoniX/subvideo/models"

	"google.golang.org/api/googleapi/transport"
	youtube "google.golang.org/api/youtube/v3"
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
	client   *http.Client
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

func (clientVideo ClientVideo) SortVideo(user models.User, n int, channelID string, page int) (subVideos []models.Subvideo, err error) {
	subVideos, err = models.SelectVideo(int(user.Id), n, channelID, page)
	if err != nil {
		return subVideos, err
	}

	return subVideos, nil
}

func (clientVideo ClientVideo) TWGetVideo(user models.User) (err error) {
	videos := clientVideo.twClient.GetVideos(user.TWOAuth)
	for _, video := range videos {
		video.UserID = user.Id
		video.Insert()
	}

	return nil
}

func (clientVideo ClientVideo) YTGetVideo(user models.User) (err error) {
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
		var ids string
		channelID := item.Snippet.ResourceId.ChannelId

		callSearchVideos := clientVideo.ytClient.Search.List("snippet").
			Q("").
			ChannelId(channelID).
			MaxResults(5).
			Order("date").
			Type("video")

		responseSearchVideos, err := callSearchVideos.Do()
		if err != nil {
			return err
		}

		for _, videoSearch := range responseSearchVideos.Items {
			ids += videoSearch.Id.VideoId + ","
		}
		callVideos := clientVideo.ytClient.Videos.List("snippet,contentDetails").Id(ids)
		responseVideos, err := callVideos.Do()
		if err != nil {
			return err
		}

		for _, video := range responseVideos.Items {
			ytTime, err := time.Parse(time.RFC3339, video.Snippet.PublishedAt)
			if err != nil {
				return err
			}
			durationVideo, err := time.ParseDuration(strings.ToLower(video.ContentDetails.Duration[2:]))
			if err != nil {
				return err
			}

			subvideo := models.Subvideo{
				UserID:      user.Id,
				TypeSub:     "youtube",
				Title:       video.Snippet.Title,
				Channel:     video.Snippet.ChannelTitle,
				ChannelID:   video.Snippet.ChannelId,
				Description: video.Snippet.Description,
				URL:         "https://www.youtube.com/watch?v=" + video.Id,
				ThumbURL:    video.Snippet.Thumbnails.High.Url,
				Length:      int(durationVideo.Seconds()),
				Date:        ytTime.UTC(),
			}
			subvideo.Insert()

			if err != nil {
				return err
			}

		}
	}

	return nil
}
