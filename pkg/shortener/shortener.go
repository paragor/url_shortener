package shortener

import (
	"context"
	"time"
)

type ShortenerService interface {
	GenerateShortUrl(ctx context.Context, now time.Time, longUrl string, ttl time.Duration) (string, error)
	GetLongUrl(ctx context.Context, now time.Time, shortUrl string) (string, error)
	Migrations(ctx context.Context) error
}
