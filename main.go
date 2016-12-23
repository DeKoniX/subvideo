package main

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"

	"strconv"

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
	Secret  string `yaml:"secret"`
	HeadURL string `yaml:"headurl"`
}

var funcMap = template.FuncMap{
	"split":    split,
	"timeZone": timeZone,
	"videoLen": videoLen,
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

	fs := http.FileServer(http.Dir("./view/static"))
	http.Handle("/static/", http.StripPrefix("/static", fs))
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/last", lastHandler)
	http.HandleFunc("/oauth/twitch", twOAuthHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/user", userHandler)
	http.HandleFunc("/user/change", userChangeHandler)
	http.HandleFunc("/logout", logoutHandler)
	http.HandleFunc("/favicon.ico", faviconHandler)

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
	http.Redirect(w, r, "/", 302)
}

func twOAuthHandler(w http.ResponseWriter, r *http.Request) {
	code := r.FormValue("code")
	oauth := clientVideo.twClient.Auth(code)
	user, err := clientVideo.twClient.OAuthTest(oauth)
	if err != nil {
		log.Println(err)
		http.Redirect(w, r, "/login", 302)
	}
	date := time.Now().UTC()
	hash := crypt(user.UserName, date)

	err = clientVideo.dataBase.InsertUser(
		user.YTChannelID,
		user.TWChannelID,
		oauth,
		user.UserName,
		user.AvatarURL,
		user.TimeZone,
		hash,
		date,
	)
	if err != nil {
		log.Panic(err)
	}
	users, err := clientVideo.dataBase.SelectUserForUserName(user.UserName)
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
	http.Redirect(w, r, "/", 302)
}

func loginHandler(w http.ResponseWriter, _ *http.Request) {
	type temp struct {
		HeadURL string
		URL     string
	}

	u, _ := url.Parse("https://api.twitch.tv/kraken/oauth2/authorize")
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", clientVideo.twClient.ClientID)
	q.Set("scope", "user_read")
	q.Set("redirect_uri", config.Twitch.RedirectURI)
	u.RawQuery = q.Encode()

	t, _ := template.ParseFiles("./view/login.html")
	t.Execute(w, temp{HeadURL: config.HeadURL, URL: u.String()})
}

func faviconHandler(w http.ResponseWriter, _ *http.Request) {
	file, _ := ioutil.ReadFile("view/favicon.ico")
	fmt.Fprint(w, string(file))
}

func currentUser(r *http.Request) (_ User) {
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
	users, err := clientVideo.dataBase.SelectUserForUserName(username)
	if err != nil {
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

func lastHandler(w http.ResponseWriter, r *http.Request) {
	var page, pageNext, pageLast int

	pageS := r.FormValue("page")
	page, err := strconv.Atoi(pageS)
	if err != nil || page == 0 {
		page = 1
	}
	pageNext = page + 1
	pageLast = page - 1

	channelID := r.FormValue("channelID")

	user := currentUser(r)

	if user.UserName != "" {
		var title string
		type pageStruct struct {
			Page int
			Next int
			Last int
		}
		type temp struct {
			Title     string
			SubVideos []SubVideo
			User      User
			Page      pageStruct
		}

		subVideos, err := clientVideo.SortVideo(user, 42, channelID, page)
		if len(subVideos) == 0 {
			http.Redirect(w, r, "/last?channelID="+channelID+"&page="+strconv.Itoa(pageLast), 302)
			return
		}
		if err != nil {
			log.Panicln(err)
		}
		title = fmt.Sprintf("%s последние видео", subVideos[0].Channel)
		t := template.Must(template.New("last.html").Funcs(funcMap).ParseFiles("./view/last.html"))
		t.Execute(w, temp{
			Title:     title,
			SubVideos: subVideos,
			User:      user,
			Page: pageStruct{
				Page: page,
				Next: pageNext,
				Last: pageLast,
			},
		})
	} else {
		http.Redirect(w, r, "/login", 302)
		return
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	var page, pageNext, pageLast int

	pageS := r.FormValue("page")
	page, err := strconv.Atoi(pageS)
	if err != nil || page == 0 {
		page = 1
	}
	pageNext = page + 1
	pageLast = page - 1

	user := currentUser(r)

	if user.UserName != "" {
		var title string
		clientVideo.twClient.GetOnline(user.TWOAuth)
		type pageStruct struct {
			Page int
			Next int
			Last int
		}
		type temp struct {
			Title         string
			SubVideos     []SubVideo
			ChannelOnline []SubVideo
			User          User
			Page          pageStruct
		}

		subVideos, err := clientVideo.SortVideo(user, 42, "", page)
		if err != nil {
			log.Panicln(err)
		}
		channelOnline := clientVideo.twClient.GetOnline(user.TWOAuth)
		switch len(channelOnline) {
		case 1:
			title = fmt.Sprintf("сейчас идет %d стрим", len(channelOnline))
		case 2, 3, 4:
			title = fmt.Sprintf("сейчас идет %d стрима", len(channelOnline))
		default:
			title = fmt.Sprintf("сейчас идет %d стримов", len(channelOnline))
		}
		if page != 1 {
			title += fmt.Sprintf(", страница %d", page)
		}

		t := template.Must(template.New("index.html").Funcs(funcMap).ParseFiles("./view/index.html"))
		t.Execute(w, temp{
			Title:         title,
			SubVideos:     subVideos,
			ChannelOnline: channelOnline,
			User:          user,
			Page: pageStruct{
				Page: page,
				Next: pageNext,
				Last: pageLast,
			},
		})
	} else {
		http.Redirect(w, r, "/login", 302)
		return
	}
}

func userHandler(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)

	if user.UserName != "" {
		var title string
		type temp struct {
			Title     string
			User      User
			TimeZones timeZones
		}

		title = fmt.Sprintf("Настройки пользователя %s", user.UserName)
		t, _ := template.ParseFiles("./view/user.html")
		t.Execute(w, temp{
			Title:     title,
			User:      user,
			TimeZones: getTimeZones(),
		})
	} else {
		http.Redirect(w, r, "/login", 302)
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
		timezone := r.FormValue("timezone")

		err := clientVideo.dataBase.InsertUser(
			ytChannelID,
			user.TWChannelID,
			user.TWOAuth,
			user.UserName,
			user.AvatarURL,
			timezone,
			user.Crypt,
			user.Date,
		)
		if err != nil {
			log.Panic(err)
		}

		user = currentUser(r)
		go runUser(user)
		http.Redirect(w, r, "/", 302)
	} else {
		http.Redirect(w, r, "/login", 302)
	}
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
				err = clientVideo.dataBase.DeleteVideoWhereInterval(10)
				if err != nil {
					log.Println("ERR Clear Video: ", err)
				}
				err = clientVideo.dataBase.DeleteUserWhereInterval(30)
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

func crypt(username string, dateChange time.Time) string {
	h := md5.New()
	io.WriteString(h, username)
	io.WriteString(h, config.Secret)
	io.WriteString(h, dateChange.Format(time.Stamp))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func cryptTest(username, hash string, dateChange time.Time) bool {
	h := md5.New()
	io.WriteString(h, username)
	io.WriteString(h, config.Secret)
	io.WriteString(h, dateChange.Format(time.Stamp))
	thisHash := fmt.Sprintf("%x", h.Sum(nil))
	return thisHash == hash
}

func split(a, b int) bool {
	return a%b == 0
}

func timeZone(t time.Time, tz string) (timeString string) {
	timezone, _ := time.LoadLocation(tz)
	ttz := t.In(timezone)
	timeString = ttz.Format("02-01-06 ------ 15:04")
	return timeString
}

type timeZones []struct {
	Offset int    `json:"offset"`
	UTC    string `json:"utc"`
}

func getTimeZones() (timeZones timeZones) {
	dat, _ := ioutil.ReadFile("timezones.json")
	json.Unmarshal(dat, &timeZones)

	return timeZones
}

func videoLen(len int) (strLength string) {
	var hour, min, second int
	if len > 60 {
		min = len / 60
		second = len % 60

		if min > 59 {
			hour = min / 60
			min = min % 60

			strLength = fmt.Sprintf("Часов: %d, Минуты: %d, ", hour, min)
		} else {
			strLength = fmt.Sprintf("Минуты: %d, ", min)
		}
		strLength = strLength + fmt.Sprintf("Секунды: %d", second)
	} else {
		strLength = fmt.Sprintf("Секунды: %d", len)
	}

	return strLength
}
