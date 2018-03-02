package main

import (
	"flag"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/DeKoniX/subvideo/models"
	"github.com/DeKoniX/subvideo/video"
	"github.com/go-macaron/binding"
	"github.com/go-macaron/gzip"
	macaron "gopkg.in/macaron.v1"
	yaml "gopkg.in/yaml.v2"
)

type configYML struct {
	YouTube struct {
		ClientID     string `yaml:"clientid"`
		ClientSecret string `yaml:"clientsecret"`
		RedirectURI  string `yaml:"redirecturi"`
		DeveloperKey string `yaml:"developerkey"`
	}
	Twitch struct {
		ClientID     string `yaml:"clientid"`
		ClientSecret string `yaml:"clientsecret"`
		RedirectURI  string `yaml:"redirecturi"`
	}
	DataBase struct {
		Host     string `yaml:"host"`
		Port     string `yaml:"port"`
		DBname   string `yaml:"dbname"`
		UserName string `yaml:"username"`
		Password string `yaml:"password"`
	}
	Secret              string `yaml:"secret"`
	HeadURL             string `yaml:"headurl"`
	DeleteVideoInterval int    `yaml:"delete_video_interval"`
	DeleteUserInterval  int    `yaml:"delete_user_interval"`
	Metrics             struct {
		Yandex int    `yaml:"yandex"`
		Google string `yaml:"google"`
	}
}

var config configYML

var clientVideo *video.ClientVideo

func main() {
	var configPath = flag.String("config", "subvideo.yml", "Путь до конфигурационного файла")
	flag.Parse()

	err := getConfig(*configPath)
	if err != nil {
		log.Panic(err)
	}

	clientVideo = video.Init(
		config.Twitch.ClientID,
		config.Twitch.ClientSecret,
		config.Twitch.RedirectURI,
		config.YouTube.ClientID,
		config.YouTube.ClientSecret,
		config.YouTube.RedirectURI,
	)

	err = models.Init(config.DataBase.Host, config.DataBase.Port, config.DataBase.UserName, config.DataBase.Password, config.DataBase.DBname)
	if err != nil {
		log.Panic(err)
	}
	go runTime()

	m := macaron.Classic()
	m.Use(macaron.Renderer(macaron.RenderOptions{
		Funcs: []template.FuncMap{map[string]interface{}{
			"split":                split,
			"getTime":              getTime,
			"videoLen":             videoLen,
			"metrics":              metrics,
			"navMenu":              navMenu,
			"dateStreamLen":        dateStreamLen,
			"userTimeZoneAndVideo": userTimeZoneAndVideo,
			"minus":                minus,
			"hashFile":             hashFile,
		}},
	}))
	m.Use(macaron.Static("public"))
	m.Use(gzip.Gziper())

	m.Get("/", indexHandler)
	m.Get("/last", lastHandler)
	m.Get("/search", searchHandler)
	m.Get("/play", playHandler)
	m.Get("/oauth/twitch", twOAuthHandler)
	m.Get("/oauth/youtube", ytOAuthHandler)
	m.Get("/login", loginHandler)
	m.Get("/logout", logoutHandler)
	m.Combo("/user").
		Get(userHandler).
		Post(binding.Bind(ChangeUserForm{}), userChangeHandler)

	log.Println("Server is running...")
	log.Println(http.ListenAndServe(":8181", m))
}

func runUser(user models.User) {
	log.Println("RUN User: ", user.UserName)
	err := clientVideo.YTGetVideo(user)
	if err != nil {
		log.Println("ERR YT: ", err)
	}
	err = clientVideo.TWGetVideo(user)
	if err != nil {
		log.Println("ERR TW: ", err)
	}
}

func runTime() {
	var err error
	run := true

	users, err := models.SelectUsers()
	if err != nil {
		log.Println("ERR Users get: ", err)
	}

	log.Println("This RUN groutine")
	for _, user := range users {

		err = clientVideo.YTGetVideo(user)
		if err != nil {
			log.Println("ERR YT: ", err)
		}
		err = clientVideo.TWGetVideo(user)
		if err != nil {
			log.Println("ERR TW: ", err)
		}
	}

	for {
		if time.Now().Minute()%30 == 0 && run {
			log.Println("RUN groutine")
			run = false
			users, err = models.SelectUsers()
			if err != nil {
				log.Println("ERR Users get: ", err)
			}
			for _, user := range users {
				err := clientVideo.YTGetVideo(user)
				if err != nil {
					log.Println("ERR YT: ", err)
				}
				err = clientVideo.TWGetVideo(user)
				if err != nil {
					log.Println("ERR TW: ", err)
				}
			}
			if time.Now().Minute() == 0 {
				err = models.DeleteVideoWhereInterval(config.DeleteVideoInterval)
				if err != nil {
					log.Println("ERR Clear Video: ", err)
				}
				err = models.DeleteUserWhereInterval(config.DeleteUserInterval)
				if err != nil {
					log.Println("ERR Clear Users: ", err)
				}
			}
		} else {
			run = true
		}

		time.Sleep(time.Second * 30)
	}
}

func getConfig(configPath string) (err error) {
	dat, err := ioutil.ReadFile(configPath)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(dat, &config)
	if err != nil {
		return err
	}
	return nil
}
