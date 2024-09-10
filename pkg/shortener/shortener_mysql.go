package shortener

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"strings"
	"time"
)

type mysqlShortenerService struct {
	db             *sql.DB
	shortUrlLength int
	randomStringFn RandomString
}

func NewMysqlShortenerService(
	db *sql.DB,
	shortUrlLength int,
	randomStringFn RandomString,
) ShortenerService {
	return &mysqlShortenerService{db: db, shortUrlLength: shortUrlLength, randomStringFn: randomStringFn}
}

const maxInsertTries = 5

func (s *mysqlShortenerService) GenerateShortUrl(ctx context.Context, now time.Time, longUrl string, ttl time.Duration) (string, error) {
	var expire *time.Time
	if ttl > 0 {
		e := now.Add(ttl)
		expire = &e
	}
	const insertQuery = "INSERT INTO `short_urls` (`short_url`, `long_url`, `created_at`, `expire_at`) VALUES (?, ?, ?, ?)"
	stmt, err := s.db.PrepareContext(ctx, insertQuery)
	if err != nil {
		return "", fmt.Errorf("cant prepare sql statement: %w", err)
	}
	err = nil
	tries := 0
	for {
		if tries > maxInsertTries {
			return "", fmt.Errorf("after %d tries still have error: %w", tries, err)
		}
		shortUrl := s.randomStringFn(s.shortUrlLength)
		_, err = stmt.ExecContext(ctx, shortUrl, longUrl, now, expire)
		var mysqlErr *mysql.MySQLError
		if err != nil && errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			tries++
			continue
		}
		if err != nil {
			return "", fmt.Errorf("cant insert into mysql: %w", err)
		}
		return shortUrl, nil
	}
}

func (s *mysqlShortenerService) GetLongUrl(ctx context.Context, now time.Time, shortUrl string) (string, error) {
	const selectQuery = "SELECT `long_url`, `expire_at` FROM `short_urls` where `short_url` = ?"
	stmt, err := s.db.PrepareContext(ctx, selectQuery)
	if err != nil {
		return "", fmt.Errorf("cant prepare sql statement: %w", err)
	}
	result, err := stmt.QueryContext(ctx, shortUrl)
	if err != nil {
		return "", fmt.Errorf("cant exec sql statement: %w", err)
	}
	longUrl := ""
	expireAt := sql.NullTime{}
	for result.Next() {
		if err := result.Scan(&longUrl, &expireAt); err != nil {
			return "", fmt.Errorf("cant get result: %w", err)
		}
	}
	if expireAt.Valid && now.After(expireAt.Time) {
		return "", nil
	}

	return longUrl, nil
}
func (s *mysqlShortenerService) Migrations(ctx context.Context) error {
	createShortUrlsTable := strings.ReplaceAll(`
CREATE TABLE IF NOT EXISTS 'short_urls' (
  'short_url' VARCHAR(255) NOT NULL,
  'long_url' TEXT NOT NULL,
  'created_at' DATETIME NOT NULL,
  'expire_at' DATETIME,
  PRIMARY KEY ('short_url')
)
`, "'", "`")
	stmt, err := s.db.PrepareContext(ctx, createShortUrlsTable)
	if err != nil {
		return fmt.Errorf("cant prepare sql statement: %w", err)
	}
	if _, err := stmt.ExecContext(ctx); err != nil {
		return fmt.Errorf("cant create `short_urls` table: %w", err)
	}

	return nil
}
