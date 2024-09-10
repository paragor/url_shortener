package shortener

import (
	"context"
	"fmt"
	"github.com/paragor/url_shortener/pkg/logger"
	"github.com/stretchr/testify/assert"
	"net/http"
	"strconv"
	"testing"
	"time"
)

type fakeShortener struct {
	generateShortUrlFn func(ctx context.Context, now time.Time, longUrl string, ttl time.Duration) (string, error)
	getLongUrlFn       func(ctx context.Context, now time.Time, shortUrl string) (string, error)
	migrationsFn       func(ctx context.Context) error
}

func (s *fakeShortener) GenerateShortUrl(ctx context.Context, now time.Time, longUrl string, ttl time.Duration) (string, error) {
	return s.generateShortUrlFn(ctx, now, longUrl, ttl)
}

func (s *fakeShortener) GetLongUrl(ctx context.Context, now time.Time, shortUrl string) (string, error) {
	return s.getLongUrlFn(ctx, now, shortUrl)
}

func (s *fakeShortener) Migrations(ctx context.Context) error {
	return s.migrationsFn(ctx)
}

func TestHttpServer_handleShortUrl(t *testing.T) {
	logger.InitForTest()
	type fields struct {
		shortener ShortenerService
	}
	type args struct {
		method    string
		url       string
		expectUrl string
		want404   bool
	}
	tests := []struct {
		fields fields
		args   args
	}{
		{
			fields: fields{
				shortener: &fakeShortener{
					getLongUrlFn: func(ctx context.Context, now time.Time, shortUrl string) (string, error) {
						if shortUrl == "qwerty" {
							return "https://google.com/path", nil
						}
						return "", nil
					},
				},
			},
			args: args{
				method:    http.MethodGet,
				url:       "http://localhost/s/qwerty",
				expectUrl: "https://google.com/path",
				want404:   false,
			},
		},
		{
			fields: fields{
				shortener: &fakeShortener{
					getLongUrlFn: func(ctx context.Context, now time.Time, shortUrl string) (string, error) {
						return "", nil
					},
				},
			},
			args: args{
				method:    http.MethodGet,
				url:       "http://localhost/s/qwerty",
				expectUrl: "",
				want404:   true,
			},
		},
	}
	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			s := &HttpServer{
				shortener:      tt.fields.shortener,
				shortUrlScheme: "http",
				shortUrlDomain: "localhost",
				shortUrlPath:   "/",
			}
			if tt.args.want404 {
				assert.True(t, assert.HTTPStatusCode(t, s.handleShortUrl, tt.args.method, tt.args.url, nil, 404))
			} else {
				assert.True(t, assert.HTTPRedirect(t, s.handleShortUrl, tt.args.method, tt.args.url, nil))
				assert.HTTPBodyContains(t, s.handleShortUrl, tt.args.method, tt.args.url, nil, fmt.Sprintf(`"%s"`, tt.args.expectUrl))
			}
		})
	}
}
