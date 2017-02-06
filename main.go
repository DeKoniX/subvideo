package main

import (
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/go-macaron/binding"
	"github.com/go-macaron/gzip"

	macaron "gopkg.in/macaron.v1"
	yaml "gopkg.in/yaml.v2"
)

type configYML struct {
	YouTube struct {
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
}

var config configYML
var clientVideo ClientVideo

func main() {
	err := getConfig()
	if err != nil {
		log.Panic(err)
	}

	clientVideo = InitClientVideo(
		config.Twitch.ClientID,
		config.Twitch.ClientSecret,
		config.YouTube.DeveloperKey,
	)

	clientVideo.dataBase, err = DBInit(
		config.DataBase.Host,
		config.DataBase.Port,
		config.DataBase.UserName,
		config.DataBase.Password,
		config.DataBase.DBname,
	)
	if err != nil {
		log.Fatal("DB ERR: ", err)
	}
	go runTime()

	m := macaron.Classic()
	m.Use(macaron.Renderer(macaron.RenderOptions{
		Funcs: []template.FuncMap{map[string]interface{}{
			"split":    split,
			"getTime":  getTime,
			"videoLen": videoLen,
		}},
	}))
	m.Use(macaron.Static("public"))
	m.Use(gzip.Gziper())

	m.Get("/", indexHandler)
	m.Get("/last", lastHandler)
	m.Get("/oauth/twitch", twOAuthHandler)
	m.Get("/login", loginHandler)
	m.Get("/logout", logoutHandler)
	m.Combo("/user").
		Get(userHandler).
		Post(binding.Bind(ChangeUserForm{}), userChangeHandler)

	log.Println("Server is running...")
	log.Println(http.ListenAndServe(":8181", m))
}

func runUser(user User) {
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

	users, err := clientVideo.dataBase.SelectUsers()
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
			users, err = clientVideo.dataBase.SelectUsers()
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
				err = clientVideo.dataBase.DeleteVideoWhereInterval(config.DeleteVideoInterval)
				if err != nil {
					log.Println("ERR Clear Video: ", err)
				}
				err = clientVideo.dataBase.DeleteUserWhereInterval(config.DeleteUserInterval)
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
