package video

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"

	"errors"

	"github.com/DeKoniX/subvideo/models"
)

type TW struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	HTTPClient   *http.Client
}

func TWInit(clientID, clientSecret, redirectURI string) *TW {
	return &TW{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		HTTPClient:   &http.Client{},
		RedirectURI:  redirectURI,
	}
}

func (tw *TW) connect(url, oauth string) (body []byte) {
	req, _ := http.NewRequest("GET", "https://api.twitch.tv/kraken/"+url, nil)
	req.Header.Add("Accept", "application/vnd.twitchtv.v3+json")
	req.Header.Add("Client-ID", tw.ClientID)
	if oauth != "" {
		req.Header.Add("Authorization", "OAuth "+oauth)
	}
	resp, _ := tw.HTTPClient.Do(req)
	body, _ = ioutil.ReadAll(resp.Body)

	return body
}

func (tw *TW) OAuthTest(accessToken string) (twChannelID, userName, avatarURL string, err error) {
	body := tw.connect("user", accessToken)

	type twJSON struct {
		DisplayName string `json:"display_name"`
		Name        string `json:"name"`
		Logo        string `json:"logo"`
		Error       string `json:"error"`
		Message     string `json:"message"`
	}

	var twjson twJSON

	json.Unmarshal(body, &twjson)

	if twjson.Error != "" {
		return twChannelID, userName, avatarURL, fmt.Errorf("ERR Twitch API: %s, %s", twjson.Error, twjson.Message)
	}

	twChannelID = twjson.Name
	userName = twjson.DisplayName
	avatarURL = twjson.Logo
	return twChannelID, userName, avatarURL, nil
}

func (tw *TW) Auth(code string) string {
	resp, _ := tw.HTTPClient.PostForm("https://api.twitch.tv/kraken/oauth2/token",
		url.Values{
			"client_id":     {tw.ClientID},
			"client_secret": {tw.ClientSecret},
			"grant_type":    {"authorization_code"},
			"redirect_uri":  {tw.RedirectURI},
			"code":          {code},
		},
	)
	type jsonTW struct {
		AccessToken string `json:"access_token"`
	}

	var jsontw jsonTW
	body, _ := ioutil.ReadAll(resp.Body)
	json.Unmarshal(body, &jsontw)

	return jsontw.AccessToken
}

func (tw *TW) GetOnline(oauth string) (videos []models.Subvideo) {
	body := tw.connect("streams/followed?limit=50&stream_type=live", oauth)

	type jsonTW struct {
		Streams []struct {
			Game      string `json:"game"`
			CreatedAt string `json:"created_at"`
			Preview   struct {
				Large string `json:"large"`
			}
			Channel struct {
				Status      string `json:"status"`
				DisplayName string `json:"display_name"`
				Name        string `json:"name"`
				URL         string `json:"url"`
			}
		}
	}

	var jsontw jsonTW
	json.Unmarshal(body, &jsontw)

	for _, stream := range jsontw.Streams {
		twTime, err := time.Parse(time.RFC3339, stream.CreatedAt)
		if err != nil {
			twTime = time.Now()
		}
		videos = append(videos, models.Subvideo{
			TypeSub:   "stream",
			Title:     stream.Channel.Status,
			Channel:   stream.Channel.DisplayName,
			ChannelID: stream.Channel.Name,
			Game:      stream.Game,
			ThumbURL:  stream.Preview.Large,
			URL:       stream.Channel.URL,
			Length:    getLength(twTime),
		})
	}
	return videos
}

func (tw *TW) GetVideos(oauth string) (videos []models.Subvideo) {
	body := tw.connect("videos/followed?limit=20&broadcast_type=all", oauth)

	type jsonTW struct {
		Videos []struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			URL         string `json:"url"`
			ID          string `json:"_id"`
			RecordedAt  string `json:"recorded_at"`
			Game        string `json:"game"`
			Length      int    `json:"length"`
			Preview     string `json:"preview"`
			Channel     struct {
				Name        string `json:"name"`
				DisplayName string `json:"display_name"`
			}
		}
	}

	var jsontw jsonTW
	json.Unmarshal(body, &jsontw)

	for _, video := range jsontw.Videos {
		twTime, err := time.Parse(time.RFC3339, video.RecordedAt)
		if err != nil {
			log.Panicln(err)
		}
		if video.Length > 300 {
			videos = append(videos, models.Subvideo{
				TypeSub:     "twitch",
				Title:       video.Title,
				Channel:     video.Channel.DisplayName,
				ChannelID:   video.Channel.Name,
				Game:        video.Game,
				Description: video.Description,
				URL:         video.URL,
				VideoID:     video.ID,
				ThumbURL:    video.Preview,
				Length:      video.Length,
				Date:        twTime.UTC(),
			})
		}
	}

	return videos
}

func (tw *TW) GetChannel(oauth, channelID string) (_ models.Subvideo, err error) {
	body := tw.connect("channels/"+channelID, oauth)

	type jsonTW struct {
		Status      string `json:"status"`
		DisplayName string `json:"display_name"`
		Game        string `json:"game"`
		Name        string `json:"name"`
		URL         string `json:"url"`
		Error       string `json:"error"`
	}

	var jsontw jsonTW
	json.Unmarshal(body, &jsontw)
	if jsontw.Error != "" {
		return models.Subvideo{}, errors.New("ERR: " + channelID + ": " + jsontw.Error)
	}

	return models.Subvideo{
		TypeSub:   "stream",
		Title:     jsontw.Status,
		Channel:   jsontw.DisplayName,
		ChannelID: jsontw.Name,
		Game:      jsontw.Game,
		URL:       jsontw.URL,
	}, nil
}

func getLength(timeStream time.Time) int {
	return int(time.Now().Unix() - timeStream.Unix())
}
