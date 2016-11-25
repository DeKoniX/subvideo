package main

import (
	"database/sql"
	"fmt"
	"strconv"
	"time"

	_ "github.com/lib/pq"
)

type DB struct {
	db *sql.DB
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
        date        timestamp with time zone
		);
		`
	_, err = DataBase.db.Exec(sqlStmt)
	if err != nil {
		return DataBase, fmt.Errorf("DB Exec: %s", err)
	}

	return DataBase, nil
}

func (DataBase *DB) Insert(typeSub, title, channel, channelID, game, description, url, thumbURL string, date time.Time) (err error) {
	if DataBase.testItem(url) == false {
		tx, err := DataBase.db.Begin()
		if err != nil {
			return err
		}

		stmt, err := tx.Prepare("INSERT INTO subvideo(type, title, channel, channel_id, game, description, url, thumb_url, date) VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9)")
		if err != nil {
			return err
		}
		defer stmt.Close()

		_, err = stmt.Exec(typeSub, title, channel, channelID, game, description, url, thumbURL, date)
		if err != nil {
			return err
		}
		tx.Commit()
	}

	return nil
}

func (DataBase *DB) testItem(url string) bool {
	var id int64
	row := DataBase.db.QueryRow("SELECT id FROM subvideo WHERE url = $1", url)
	err := row.Scan(&id)
	if err != nil {
		return false
	}

	return true
}

func (DataBase *DB) Select(n int) (selectRows []SubVideo, err error) {
	rows, err := DataBase.db.Query("SELECT type, title, channel, channel_id, game, description, url, thumb_url, date FROM subvideo ORDER BY date DESC LIMIT " + strconv.Itoa(n))
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
