package main

import (
	"encoding/base64"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	yaml "gopkg.in/yaml.v2"
)

type configYML struct {
	YouTube struct {
		DeveloperKey string
		MyChannelID  string
	}
	Twitch struct {
		ClientID      string
		MyChannelName string
	}
	BasicAuth struct {
		Username string
		Password string
	}
	TimeZone struct {
		Zone string
	}
}

var config configYML
var client ClientVideo

func main() {
	err := getConfig()
	if err != nil {
		log.Panic(err)
	}

	client = InitClientVideo(
		config.Twitch.ClientID,
		config.YouTube.DeveloperKey,
		config.Twitch.MyChannelName,
		config.YouTube.MyChannelID,
	)
	client.TimeZone, err = time.LoadLocation(config.TimeZone.Zone)
	if err != nil {
		client.TimeZone, _ = time.LoadLocation("UTC")
	}

	fs := http.FileServer(http.Dir("./view/static"))
	http.Handle("/static/", http.StripPrefix("/static", fs))
	http.HandleFunc("/", basicAuth(indexHandler))
	http.HandleFunc("/favicon.png", faviconHandler)

	log.Fatal(http.ListenAndServe(":8181", nil))
}

func faviconHandler(w http.ResponseWriter, r *http.Request) {
	file, _ := ioutil.ReadFile("view/favicon.png")
	fmt.Fprint(w, string(file))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	type temp struct {
		SubVideos     []SubVideo
		ChannelOnline []ChannelOnline
	}
	subVideos, err := client.SortVideo(40)
	if err != nil {
		log.Fatal(err)
	}
	channelOnline, err := client.ChannelsOnline()
	if err != nil {
		log.Fatal(err)
	}

	t, _ := template.ParseFiles("./view/index.html")
	t.Execute(w, temp{
		SubVideos:     subVideos,
		ChannelOnline: channelOnline,
	})
}

func basicAuth(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authError := func() {
			w.Header().Set("WWW-Authenticate", "Basic realm=\"SubVideo\"")
			http.Error(w, "authorization failed", http.StatusUnauthorized)
		}
		auth := strings.SplitN(r.Header.Get("Authorization"), " ", 2)
		if len(auth) != 2 || auth[0] != "Basic" {
			authError()
			return
		}
		payload, err := base64.StdEncoding.DecodeString(auth[1])
		if err != nil {
			authError()
			return
		}
		pair := strings.SplitN(string(payload), ":", 2)
		if len(pair) != 2 || !(pair[0] == config.BasicAuth.Username && pair[1] == config.BasicAuth.Password) {
			authError()
			return
		}
		handler(w, r)
	}
}

func getConfig() (err error) {
	dat, err := ioutil.ReadFile("subvideo.yml")
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(dat, &config)
	if err != nil {
		return err
	}
	return nil
}
