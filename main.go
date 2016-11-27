package main

import (
	"crypto/md5"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"

	yaml "gopkg.in/yaml.v2"
)

type configYML struct {
	YouTube struct {
		DeveloperKey string
	}
	Twitch struct {
		ClientID     string
		ClientSecret string
		RedirectURI  string
	}
	DataBase struct {
		Host     string
		Port     string
		DBname   string
		UserName string
		Password string
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
var clientVideo ClientVideo
var tw TW

func main() {
	err := getConfig()
	if err != nil {
		log.Panic(err)
	}
	cli := &http.Client{}
	tw = TWInit(cli, config.Twitch.ClientID, config.Twitch.ClientSecret)

	clientVideo = InitClientVideo(
		config.Twitch.ClientID,
		config.YouTube.DeveloperKey,
	)
	clientVideo.TimeZone, err = time.LoadLocation(config.TimeZone.Zone)
	if err != nil {
		clientVideo.TimeZone, _ = time.LoadLocation("UTC")
	}
	clientVideo.DataBase, err = DBInit(
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

	fs := http.FileServer(http.Dir("./view/static"))
	http.Handle("/static/", http.StripPrefix("/static", fs))
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/twitch/oauth", twOAuthHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/user", userHandler)
	http.HandleFunc("/user/change", userChangeHandler)
	http.HandleFunc("/logout", logoutHandler)
	http.HandleFunc("/favicon.png", faviconHandler)

	log.Println("Listen server: :8181")
	log.Fatal(http.ListenAndServe(":8181", nil))
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:   "username",
		Value:  "",
		MaxAge: -1,
		Path:   "/",
	})
	http.SetCookie(w, &http.Cookie{
		Name:   "crypt",
		Value:  "",
		MaxAge: -1,
		Path:   "/",
	})
	http.Redirect(w, r, "/", 301)
}

func twOAuthHandler(w http.ResponseWriter, r *http.Request) {
	code := r.FormValue("code")
	oauth := tw.Auth(code)
	user, err := tw.OAuthTest(oauth)
	if err != nil {
		log.Panicln(err)
		// TODO: редирект на /
	}
	date := time.Now().UTC()
	hash := crypt(user.UserName, date)

	err = clientVideo.DataBase.InsertUser(
		user.YTChannelID,
		user.TWChannelID,
		oauth,
		user.UserName,
		user.AvatarURL,
		"UTC",
		hash,
		date,
	)
	if err != nil {
		log.Panic(err)
	}
	users, err := clientVideo.DataBase.SelectUserForUserName(user.UserName)
	if err != nil {
		log.Panic(err)
	}
	go runUser(users[0])

	http.SetCookie(w, &http.Cookie{
		Name:    "username",
		Value:   user.UserName,
		Expires: time.Now().Add(time.Hour * 24 * 30),
		Path:    "/",
	})
	http.SetCookie(w, &http.Cookie{
		Name:    "crypt",
		Value:   hash,
		Expires: time.Now().Add(time.Hour * 24 * 30),
		Path:    "/",
	})
	http.Redirect(w, r, "/", 301)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	type temp struct {
		URL string
	}

	u, _ := url.Parse("https://api.twitch.tv/kraken/oauth2/authorize")
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", tw.ClientID)
	q.Set("scope", "user_read")
	q.Set("redirect_uri", config.Twitch.RedirectURI)
	u.RawQuery = q.Encode()

	t, _ := template.ParseFiles("./view/login.html")
	t.Execute(w, temp{URL: u.String()})
}

func faviconHandler(w http.ResponseWriter, r *http.Request) {
	file, _ := ioutil.ReadFile("view/favicon.png")
	fmt.Fprint(w, string(file))
}

func currentUser(r *http.Request) (user User) {
	var username string
	var hash string

	for _, cookie := range r.Cookies() {
		if cookie.Name == "username" {
			username = cookie.Value
		}
		if cookie.Name == "crypt" {
			hash = cookie.Value
		}
	}
	if username == "" {
		return User{}
	}
	users, err := clientVideo.DataBase.SelectUserForUserName(username)
	if err != nil {
		log.Println(err)
		return User{}
	}
	if len(users) == 0 {
		return User{}
	}

	if cryptTest(username, hash, users[0].Date.UTC()) {
		return users[0]
	}
	return User{}
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	var login bool

	user := currentUser(r)
	if user.UserName != "" {
		login = true
	}

	if login {
		tw.GetOnline(user.TWOAuth)
		type temp struct {
			SubVideos     []SubVideo
			ChannelOnline []SubVideo
			User          User
		}

		subVideos, err := clientVideo.SortVideo(user, 20)
		if err != nil {
			log.Panicln(err)
		}
		channelOnline := tw.GetOnline(user.TWOAuth)

		t, _ := template.ParseFiles("./view/index.html")
		t.Execute(w, temp{
			SubVideos:     subVideos,
			ChannelOnline: channelOnline,
			User:          user,
		})
	} else {
		http.Redirect(w, r, "/login", 301)
	}
}

func userHandler(w http.ResponseWriter, r *http.Request) {
	var login bool

	user := currentUser(r)
	log.Println(user)
	if user.UserName != "" {
		login = true
	}

	if login {
		type temp struct {
			User User
		}

		t, _ := template.ParseFiles("./view/user.html")
		t.Execute(w, temp{
			User: user,
		})
	} else {
		http.Redirect(w, r, "/login", 301)
	}
}

func userChangeHandler(w http.ResponseWriter, r *http.Request) {
	var login bool

	user := currentUser(r)
	if user.UserName != "" {
		login = true
	}

	if login {
		ytChannelID := r.FormValue("yt_channel_id")

		err := clientVideo.DataBase.InsertUser(
			ytChannelID,
			user.TWChannelID,
			user.TWOAuth,
			user.UserName,
			user.AvatarURL,
			user.TimeZone,
			user.Crypt,
			user.Date,
		)
		if err != nil {
			log.Panic(err)
		}

		user = currentUser(r)
		go runUser(user)
		http.Redirect(w, r, "/", 301)
	} else {
		http.Redirect(w, r, "/login", 301)
	}
}

func runUser(user User) {
	log.Println("RUN User!")
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

	users, err := clientVideo.DataBase.SelectUsers()
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
		if time.Now().Minute()%5 == 0 && run {
			log.Println("RUN groutine")
			run = false
			users, err = clientVideo.DataBase.SelectUsers()
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

func crypt(username string, dateChange time.Time) string {
	h := md5.New()
	io.WriteString(h, username)
	io.WriteString(h, dateChange.Format(time.Stamp))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func cryptTest(username, hash string, dateChange time.Time) bool {
	h := md5.New()
	io.WriteString(h, username)
	io.WriteString(h, dateChange.Format(time.Stamp))
	thisHash := fmt.Sprintf("%x", h.Sum(nil))
	return thisHash == hash
}
