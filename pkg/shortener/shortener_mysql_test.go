package shortener

import (
	"context"
	"database/sql"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
	"testing"
	"time"
)

type databaseContainer struct {
	*mysql.MySQLContainer
	connectionString string
}

func createDatabaseContainer(ctx context.Context) (*databaseContainer, error) {
	dbContainer, err := mysql.Run(
		ctx,
		"mysql:8.0",
		mysql.WithDatabase("shorturls"),
		mysql.WithUsername("root"),
		mysql.WithPassword("Password123"),
	)
	if err != nil {
		return nil, err
	}
	connStr, err := dbContainer.ConnectionString(ctx, "parseTime=true")
	if err != nil {
		return nil, err
	}

	return &databaseContainer{
		MySQLContainer:   dbContainer,
		connectionString: connStr,
	}, nil
}

func TestNewMysqlShortenerService(t *testing.T) {
	ctx := context.Background()
	container, err := createDatabaseContainer(ctx)
	assert.NoError(t, err)
	t.Cleanup(func() {
		container.Stop(ctx, nil)
	})
	db, err := sql.Open("mysql", container.connectionString)
	assert.NoError(t, err)
	assert.NoError(t, db.Ping())
	const shortUrlLength = 5
	shortener := NewMysqlShortenerService(db, shortUrlLength, MathRandomString)
	assert.NoError(t, withTimeout(ctx, time.Second*10, func(ctx context.Context) error {
		return shortener.Migrations(ctx)
	}))
	freezNow := time.Now()
	generateUrlTtl := time.Hour
	assert.NoError(t, withTimeout(ctx, time.Second*10, func(ctx context.Context) error {
		result, err := shortener.GetLongUrl(ctx, freezNow, "https://localhost/s/something")
		if err != nil {
			return err
		}
		assert.Len(t, result, 0)
		return nil
	}))
	shortUrl := ""
	assert.NoError(t, withTimeout(ctx, time.Second*10, func(ctx context.Context) error {
		result, err := shortener.GenerateShortUrl(ctx, freezNow, "https://google.com/something", generateUrlTtl)
		if err != nil {
			return err
		}
		assert.Len(t, result, shortUrlLength)
		shortUrl = result
		return nil
	}))

	assert.NoError(t, withTimeout(ctx, time.Second*10, func(ctx context.Context) error {
		result, err := shortener.GetLongUrl(ctx, freezNow.Add(generateUrlTtl/2), shortUrl)
		if err != nil {
			return err
		}
		assert.Equal(t, "https://google.com/something", result)
		return nil
	}))
	assert.NoError(t, withTimeout(ctx, time.Second*10, func(ctx context.Context) error {
		result, err := shortener.GetLongUrl(ctx, freezNow.Add(generateUrlTtl+time.Minute), shortUrl)
		if err != nil {
			return err
		}
		assert.Equal(t, "", result)
		return nil
	}))

}

func withTimeout(ctx context.Context, timeout time.Duration, fn func(ctx context.Context) error) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return fn(ctx)
}
