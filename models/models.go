package models

import (
	"fmt"

	"github.com/go-xorm/xorm"
	_ "github.com/lib/pq"
)

var (
	x   *xorm.Engine
	err error
)

func Init(host, port, username, password, dbname string) (err error) {
	pgurl := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", username, password, host, port, dbname)
	x, err = xorm.NewEngine("postgres", pgurl)
	if err != nil {
		return err
	}
	//x.ShowSQL(true)
	err = x.Sync(new(User))
	if err != nil {
		return err
	}
	err = x.Sync(new(Subvideo))
	if err != nil {
		return err
	}

	results, err := x.Query("SELECT column_name FROM INFORMATION_SCHEMA.COLUMNS WHERE table_name = ? AND column_name = ?", "subvideo", "tsv")
	if err != nil {
		return err
	}
	if len(results) == 0 {
		_, err = x.Exec("ALTER TABLE subvideo ADD COLUMN tsv tsvector")
		if err != nil {
			return err
		}
		_, err = x.Exec("UPDATE subvideo SET tsv = " +
			"setweight(to_tsvector(coalesce(title, '')), 'A') ||" +
			"setweight(to_tsvector(coalesce(channel, '')), 'B') ||" +
			"setweight(to_tsvector(coalesce(game, '')), 'C') ||" +
			"setweight(to_tsvector(coalesce(description, '')), 'D')")
		if err != nil {
			return err
		}
		_, err = x.Exec("CREATE INDEX ix_subvideo_tsv ON subvideo USING GIN(tsv)")
		if err != nil {
			return err
		}
		_, err = x.Exec(
			`CREATE FUNCTION subvideo_trigger() RETURNS trigger AS $$
			begin
				new.tsv :=
				setweight(to_tsvector(coalesce(new.title, '')),
					'A') ||
				setweight(to_tsvector(coalesce(new.channel, '')),
						'B') ||
				setweight(to_tsvector(coalesce(new.game, '')),
						'C') ||
				setweight(to_tsvector(coalesce(new.description, '')),
						'D');
				return new;
			end
			$$ LANGUAGE plpgsql;

			CREATE TRIGGER tsvectorupdate BEFORE INSERT OR UPDATE
			ON subvideo FOR EACH ROW EXECUTE PROCEDURE subvideo_trigger();`,
		)
	}

	return nil
}
