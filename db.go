package main

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

type DB struct {
	db *sql.DB
}

type User struct {
	ID          int
	YTChannelID string
	TWChannelID string
	TWOAuth     string
	UserName    string
	AvatarURL   string
	Crypt       string
	Date        time.Time
	TimeZone    string
}

func DBInit(host, port, username, password, dbname string) (DataBase DB, err error) {
	DataBase.db, err = sql.Open("postgres",
		"postgres://"+username+":"+password+"@"+host+":"+port+"/"+dbname+"?sslmode=disable",
	)

	if err != nil {
		return DataBase, err
	}

	sqlStmt := `
		CREATE TABLE IF NOT EXISTS subvideo (
				id          SERIAL PRIMARY KEY,
        type        character varying(255),
        title       character varying(255),
        channel     character varying(255),
        channel_id  character varying(255),
        game        character varying(255),
        description text,
        url 				character varying(255),
        thumb_url   character varying(255),
        date        timestamp with time zone,
				user_id 		integer NOT NULL
		);
		`
	_, err = DataBase.db.Exec(sqlStmt)
	if err != nil {
		return DataBase, fmt.Errorf("DB Exec: %s", err)
	}

	sqlStmt = `
		CREATE TABLE IF NOT EXISTS users (
			id 							SERIAL PRIMARY KEY,
			yt_channel_id 	character varying(255),
			tw_channel_id 	character varying(255),
			tw_oauth 				character varying(255),
			username 				character varying(255),
			avatar_url 			character varying(255),
			crypt 					character varying(255),
			timezone 				character varying(255),
			date_change 		timestamp with time zone,
			date_create 		timestamp with time zone
		);
	`

	_, err = DataBase.db.Exec(sqlStmt)
	if err != nil {
		return DataBase, fmt.Errorf("DB Exec: %s", err)
	}

	return DataBase, nil
}

func (DataBase *DB) InsertUser(ytChannelID, twChannelID, twOAuth, username, avatarURL, timezone, crypt string, date time.Time) (err error) {
	users, err := DataBase.SelectUserForUserName(username)
	if err != nil {
		return err
	}
	tx, err := DataBase.db.Begin()
	if err != nil {
		return err
	}

	if len(users) > 0 {
		stmt, err := tx.Prepare(`
			UPDATE users
			SET yt_channel_id=$1 , tw_channel_id=$2, tw_oauth=$3, avatar_url=$4, crypt=$5, timezone=$6, date_change=$7
			WHERE id=$8
		`)
		if err != nil {
			return err
		}
		defer stmt.Close()

		_, err = stmt.Exec(ytChannelID, twChannelID, twOAuth, avatarURL, crypt, timezone, date.UTC(), users[0].ID)
		if err != nil {
			return err
		}
		tx.Commit()

		return nil
	}
	stmt, err := tx.Prepare(`
		INSERT INTO users(yt_channel_id, tw_channel_id, tw_oauth, username, avatar_url, crypt, timezone, date_create, date_change)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(ytChannelID, twChannelID, twOAuth, username, avatarURL, crypt, timezone, time.Now().UTC(), date.UTC())
	if err != nil {
		return err
	}
	tx.Commit()

	return nil

}

func (DataBase *DB) SelectUsers() (users []User, err error) {
	rows, err := DataBase.db.Query("SELECT id, yt_channel_id, tw_channel_id, tw_oauth, username, avatar_url, crypt, timezone, date_change FROM users")
	if err != nil {
		return users, err
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var ytChannelID, twChannelID, twOAuth, username, avatarURL, crypt, timezone string
		var date time.Time
		err = rows.Scan(&id, &ytChannelID, &twChannelID, &twOAuth, &username, &avatarURL, &crypt, &timezone, &date)
		if err != nil {
			return users, err
		}

		timeLocation, err := time.LoadLocation(timezone)
		if err != nil {
			timeLocation, _ = time.LoadLocation("UTC")
		}

		users = append(users, User{
			ID:          id,
			YTChannelID: ytChannelID,
			TWChannelID: twChannelID,
			TWOAuth:     twOAuth,
			UserName:    username,
			AvatarURL:   avatarURL,
			Crypt:       crypt,
			TimeZone:    timeLocation.String(),
			Date:        date,
		})
	}

	return users, nil
}

func (DataBase *DB) SelectUserForUserName(name string) (users []User, err error) {
	rows, err := DataBase.db.Query("SELECT id, yt_channel_id, tw_channel_id, tw_oauth, username, avatar_url, crypt, timezone, date_change FROM users WHERE username = $1", name)
	if err != nil {
		return users, err
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var ytChannelID, twChannelID, twOAuth, username, avatarURL, crypt, timezone string
		var date time.Time
		err = rows.Scan(&id, &ytChannelID, &twChannelID, &twOAuth, &username, &avatarURL, &crypt, &timezone, &date)
		if err != nil {
			return users, err
		}

		timeLocation, err := time.LoadLocation(timezone)
		if err != nil {
			timeLocation, _ = time.LoadLocation("UTC")
		}

		users = append(users, User{
			ID:          id,
			YTChannelID: ytChannelID,
			TWChannelID: twChannelID,
			TWOAuth:     twOAuth,
			UserName:    username,
			AvatarURL:   avatarURL,
			Crypt:       crypt,
			TimeZone:    timeLocation.String(),
			Date:        date,
		})
	}

	return users, nil
}

func (DataBase *DB) InsertVideo(userID int, typeSub, title, channel, channelID, game, description, url, thumbURL string, date time.Time) (err error) {
	if DataBase.testItem(url, userID) == false {
		tx, err := DataBase.db.Begin()
		if err != nil {
			return err
		}

		stmt, err := tx.Prepare("INSERT INTO subvideo(type, title, channel, channel_id, game, description, url, thumb_url, date, user_id) VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)")
		if err != nil {
			return err
		}
		defer stmt.Close()

		_, err = stmt.Exec(typeSub, title, channel, channelID, game, description, url, thumbURL, date, userID)
		if err != nil {
			return err
		}
		tx.Commit()
	}

	return nil
}

func (DataBase *DB) SelectVideo(userID, n int, channelID string) (selectRows []SubVideo, err error) {
	var rows *sql.Rows
	if channelID == "" {
		rows, err = DataBase.db.Query("SELECT type, title, channel, channel_id, game, description, url, thumb_url, date FROM subvideo WHERE user_id=$1 ORDER BY date DESC LIMIT $2", userID, n)
	} else {
		rows, err = DataBase.db.Query("SELECT type, title, channel, channel_id, game, description, url, thumb_url, date FROM subvideo WHERE user_id=$1 AND channel_id=$2 ORDER BY date DESC LIMIT $3", userID, channelID, n)
	}

	if err != nil {
		return selectRows, err
	}
	defer rows.Close()

	for rows.Next() {
		var typeSub, title, channel, channelID, game, description, url, thumbURL string
		var date time.Time
		err = rows.Scan(&typeSub, &title, &channel, &channelID, &game, &description, &url, &thumbURL, &date)
		if err != nil {
			return selectRows, err
		}

		selectRows = append(selectRows, SubVideo{
			TypeSub:     typeSub,
			Title:       title,
			Channel:     channel,
			ChannelID:   channelID,
			Description: description,
			URL:         url,
			ThumbURL:    thumbURL,
			Date:        date,
		})
	}

	return selectRows, nil
}

func (DataBase *DB) testItem(url string, userID int) bool {
	var id int64
	row := DataBase.db.QueryRow("SELECT id FROM subvideo WHERE url=$1 AND user_id=$2", url, userID)
	err := row.Scan(&id)
	if err != nil {
		return false
	}

	return true
}
