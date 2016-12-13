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
        length 			integer,
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

func (DataBase *DB) InsertVideo(userID int, typeSub, title, channel, channelID, game, description, url, thumbURL string, length int, date time.Time) error {
	b, user := DataBase.testItem(url, userID)
	tx, err := DataBase.db.Begin()
	if err != nil {
		return err
	}
	if b == false {
		stmt, err := tx.Prepare("INSERT INTO subvideo(type, title, channel, channel_id, game, description, url, thumb_url, length, date, user_id) VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)")
		if err != nil {
			return err
		}
		defer stmt.Close()

		_, err = stmt.Exec(typeSub, title, channel, channelID, game, description, url, thumbURL, length, date, userID)
		if err != nil {
			return err
		}
		tx.Commit()
	} else {
		if user == userID {
			stmt, err := tx.Prepare(`
			UPDATE subvideo
			SET title=$1 , game=$2, description=$3, thumb_url=$4, length=$5, date=$6
			WHERE url=$7
		`)
			if err != nil {
				return err
			}
			defer stmt.Close()

			_, err = stmt.Exec(title, game, description, thumbURL, length, date.UTC(), url)
			if err != nil {
				return err
			}
			tx.Commit()
		}
	}

	return nil
}

func (DataBase *DB) SelectVideo(userID, n int, channelID string, page int) (selectRows []SubVideo, err error) {
	var rows *sql.Rows
	if channelID == "" {
		rows, err = DataBase.db.Query("SELECT type, title, channel, channel_id, game, description, url, thumb_url, length, date FROM subvideo WHERE user_id=$1 ORDER BY date DESC LIMIT $2 OFFSET $3", userID, n, page*n-n)
	} else {
		rows, err = DataBase.db.Query("SELECT type, title, channel, channel_id, game, description, url, thumb_url, length, date FROM subvideo WHERE user_id=$1 AND channel_id=$2 ORDER BY date DESC LIMIT $3 OFFSET $4", userID, channelID, n, page*n-n)
	}

	if err != nil {
		return selectRows, err
	}
	defer rows.Close()

	for rows.Next() {
		var typeSub, title, channel, channelID, game, description, url, thumbURL string
		var length int
		var date time.Time
		err = rows.Scan(&typeSub, &title, &channel, &channelID, &game, &description, &url, &thumbURL, &length, &date)
		if err != nil {
			return selectRows, err
		}

		selectRows = append(selectRows, SubVideo{
			TypeSub:     typeSub,
			Title:       title,
			Channel:     channel,
			ChannelID:   channelID,
			Game:        game,
			Description: description,
			URL:         url,
			ThumbURL:    thumbURL,
			Length:      length,
			Date:        date,
		})
	}

	return selectRows, nil
}

func (DataBase *DB) DeleteVideoWhereInterval(day int) (err error) {
	duration := time.Hour * time.Duration(24*day)
	dateInterval := time.Now().Add(-duration)
	dateInterval.Format(time.RFC3339)
	_, err = DataBase.db.Exec("DELETE FROM subvideo WHERE date<$1", dateInterval)
	if err != nil {
		return err
	}

	return nil
}

func (DataBase *DB) DeleteUserWhereInterval(day int) (err error) {
	duration := time.Hour * time.Duration(24*day)
	dateInterval := time.Now().Add(-duration)
	dateInterval.Format(time.RFC3339)

	rows, err := DataBase.db.Query("SELECT id FROM users WHERE date_change<$1", dateInterval)
	if err != nil {
		return err
	}
	type usersID struct {
		ID int
	}
	var users []usersID
	for rows.Next() {
		var id int
		err = rows.Scan(&id)
		if err != nil {
			return err
		}
		users = append(users, usersID{ID: id})
	}
	rows.Close()

	for _, id := range users {
		_, err := DataBase.db.Exec("DELETE FROM subvideo WHERE user_id=$1", id.ID)
		if err != nil {
			return err
		}
		_, err = DataBase.db.Exec("DELETE FROM users WHERE id=$1", id.ID)
		if err != nil {
			return err
		}
	}

	return nil
}

func (DataBase *DB) testItem(url string, userID int) (_ bool, user int) {
	var id int

	row := DataBase.db.QueryRow("SELECT id, user_id FROM subvideo WHERE url=$1 AND user_id=$2", url, userID)
	err := row.Scan(&id, &user)
	if err != nil {
		return false, userID
	}

	return true, user
}
