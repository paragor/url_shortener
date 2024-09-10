package shortener

import (
	"encoding/json"
	"fmt"
	"github.com/paragor/url_shortener/pkg/logger"
	"go.uber.org/zap"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

type HttpServer struct {
	shortener ShortenerService

	shortUrlScheme string
	shortUrlDomain string
	shortUrlPath   string
}

func NewHttpServer(shortener ShortenerService, shortUrlScheme string, shortUrlDomain string, shortUrlPath string) (http.Handler, error) {
	if shortUrlScheme != "http" && shortUrlScheme != "https" {
		return nil, fmt.Errorf("invalid schema")
	}
	if strings.Contains(shortUrlDomain, "/") || shortUrlDomain == "" {
		return nil, fmt.Errorf("invalid domain")
	}
	if !strings.HasPrefix(shortUrlPath, "/") {
		return nil, fmt.Errorf("invalid path")
	}
	shortUrlPath = strings.ReplaceAll(shortUrlPath, "//", "/")
	return &HttpServer{
		shortener:      shortener,
		shortUrlDomain: shortUrlDomain,
		shortUrlScheme: shortUrlScheme,
		shortUrlPath:   shortUrlPath,
	}, nil
}

func (s *HttpServer) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/api/v1/generate_short_url" {
		if request.Method != "PUT" && request.Method != "POST" {
			http.Error(writer, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		s.handleGenerateUrl(writer, request)
		return
	}
	s.handleShortUrl(writer, request)
}

type GenerateUrlRequest struct {
	LongUrl    string `json:"long_url"`
	TtlSeconds int    `json:"ttl_seconds,omitempty"`
}

func (s *HttpServer) handleGenerateUrl(w http.ResponseWriter, r *http.Request) {
	log := logger.FromCtx(r.Context()).
		With(zap.String("component", "HttpServer.handleGenerateUrl"))
	var (
		longUrl string
		ttl     time.Duration
	)

	switch {
	case r.Header.Get("Content-Type") == "application/x-www-form-urlencoded":
		_ = r.ParseForm()
		longUrl = r.Form.Get("long_url")
		ttlRaw := r.Form.Get("ttl_seconds")
		if len(ttlRaw) > 0 {
			ttlSeconds, err := strconv.Atoi(ttlRaw)
			if err != nil {
				log.With(zap.Error(err)).Warn("ttl_seconds is invalid")
				http.Error(w, "ttl_seconds is invalid", http.StatusBadRequest)
				return
			}
			ttl = time.Second * time.Duration(ttlSeconds)
		}
	case strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data"):
		_ = r.ParseMultipartForm(1 * 1024 * 1024)
		longUrl = r.Form.Get("long_url")
		ttlRaw := r.Form.Get("ttl_seconds")
		if len(ttlRaw) > 0 {
			ttlSeconds, err := strconv.Atoi(ttlRaw)
			if err != nil {
				log.With(zap.Error(err)).Warn("ttl_seconds is invalid")
				http.Error(w, "ttl_seconds is invalid", http.StatusBadRequest)
				return
			}
			ttl = time.Second * time.Duration(ttlSeconds)
		}
	default:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.With(zap.Error(err)).Warn("cant read request body")
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		parsedBody := &GenerateUrlRequest{}
		if err := json.Unmarshal(body, parsedBody); err != nil {
			log.With(zap.Error(err)).Warn("cant json parse request body")
			http.Error(w, "cant parse request body: "+err.Error(), http.StatusBadRequest)
			return
		}
		longUrl = parsedBody.LongUrl
		ttl = time.Second * time.Duration(parsedBody.TtlSeconds)
	}
	if len(longUrl) == 0 {
		log.Warn("long_url is empty")
		http.Error(w, "long_url cannot be empty", http.StatusBadRequest)
		return
	}
	if ttl < 0 {
		log.Warn("ttl < 0")
		http.Error(w, "ttl_seconds should be > 0", http.StatusBadRequest)
		return
	}
	if _, err := url.Parse(longUrl); err != nil {
		log.With(zap.Error(err)).Warn("long_url is invalid")
		http.Error(w, "long_url is invalid url: "+err.Error(), http.StatusBadRequest)
		return
	}

	log.With(zap.String("long_url", longUrl)).
		With(zap.Duration("ttl", ttl))

	shortUrlEnd, err := s.shortener.GenerateShortUrl(r.Context(), time.Now(), longUrl, ttl)
	if err != nil {
		log.With(zap.Error(err)).Error("cant generate short url")
		http.Error(w, "error on generate short url", http.StatusInternalServerError)
		return
	}
	shortUrl := fmt.Sprintf("%s://%s%s%s", s.shortUrlScheme, s.shortUrlDomain, s.shortUrlPath, shortUrlEnd)
	log.With(zap.String("short_url", shortUrl)).Info("short url generation done!")

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(200)
	_, _ = w.Write([]byte(fmt.Sprintf(`{"short_url": "%s"}`, shortUrl)))
}

func (s *HttpServer) handleShortUrl(w http.ResponseWriter, r *http.Request) {
	shortUrl := path.Base(r.URL.Path)
	log := logger.FromCtx(r.Context()).
		With(zap.String("component", "HttpServer.handleShortUrl")).
		With(zap.String("short_url", shortUrl))

	longUrl, err := s.shortener.GetLongUrl(r.Context(), time.Now(), shortUrl)
	if err != nil {
		log.With(zap.Error(err)).Error("cant get short url")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	log = log.With(zap.String("long_url", longUrl))
	if longUrl == "" {
		log.Info("long url is not found")
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	http.Redirect(w, r, longUrl, http.StatusMovedPermanently)
}
