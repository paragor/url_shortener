package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	_ "github.com/go-sql-driver/mysql"
	"github.com/paragor/url_shortener/pkg/logger"
	"github.com/paragor/url_shortener/pkg/shortener"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"net/http"
	"net/http/pprof"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

func getLogLevel() zapcore.Level {
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "info", "":
		return zapcore.InfoLevel
	case "debug":
		return zapcore.DebugLevel
	case "error":
		return zapcore.ErrorLevel
	case "warn":
		return zapcore.WarnLevel
	default:
		panic("unknown logger level")
	}
}

var app = "url_shortener"

var (
	shortUrlScheme = flag.String("short-url-scheme", "https", "short url scheme: http or https")
	shortUrlDomain = flag.String("short-url-domain", "", "short url domain")
	shortUrlPath   = flag.String("short-url-path", "/", "short url path")
	shortUrlSize   = flag.Int("short-url-size", 5, "size of symbols of short url")

	listenAddr     = flag.String("listen-addr", ":8080", "listen addr")
	diagnosticAddr = flag.String("diagnostic-addr", ":7070", "diagnostic addr")

	mysqlDsn = flag.String("mysql-dsn", envWithDefault("MYSQL_DSN", "user:password@tcp(127.0.0.1:3306)/dbname?parseTime=true"), "mysql dsn (env MYSQL_DSN)")

	runMigrations = flag.Bool("run-migration", false, "run migrations")
)

func envWithDefault(name string, defaultValue string) string {
	value := os.Getenv(name)
	if value == "" {
		return defaultValue
	}
	return value
}

func main() {
	logLevel := getLogLevel()
	logger.Init(app, logLevel)
	log := logger.Logger()
	flag.Parse()
	db, err := sql.Open("mysql", *mysqlDsn)
	defer db.Close()
	if err != nil {
		log.With(zap.Error(err)).Fatal("invalid dsn")
	}
	if err := db.Ping(); err != nil {
		log.With(zap.Error(err)).Fatal("database is not respond")
	}

	shortenerService := shortener.NewMysqlShortenerService(
		db,
		*shortUrlSize,
		shortener.MathRandomString,
	)
	if *runMigrations {
		log.Info("start db migration")
		if err := shortenerService.Migrations(context.Background()); err != nil {
			log.With(zap.Error(err)).Fatal("cant run db migration")
		}
		log.Info("finish db migration")
		return
	}
	httpHandler, err := shortener.NewHttpServer(
		shortenerService,
		*shortUrlScheme,
		*shortUrlDomain,
		*shortUrlPath,
	)
	if err != nil {
		log.With(zap.Error(err)).Fatal("cant init http handler")
	}
	httpHandler = logger.HttpRecoveryMiddleware(httpHandler)
	httpHandler = logger.HttpSetLoggerMiddleware(httpHandler)

	mainServer := http.Server{
		Addr:    *listenAddr,
		Handler: httpHandler,
	}

	diagnosticServer := http.Server{
		Addr:    *diagnosticAddr,
		Handler: GetDiagnosticServerHandler(db),
	}

	diagnosticServer.RegisterOnShutdown(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
		defer cancel()
		_ = mainServer.Shutdown(ctx)
	})
	mainServer.RegisterOnShutdown(func() {
		_ = diagnosticServer.Close()
	})

	go func() {
		time.Sleep(time.Second * 5)
		log.Debug("starting diagnostic server")
		if err := diagnosticServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.With(zap.Error(err)).Error("on listen diagnostic server")
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
		defer cancel()
		_ = mainServer.Shutdown(ctx)
	}()
	log.Debug("starting main server")
	if err := mainServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.With(zap.Error(err)).Fatal("on listen main server")
	}
	log.Info("good bye")
}

func GetDiagnosticServerHandler(db *sql.DB) http.Handler {
	mux := http.NewServeMux()
	alive := atomic.Bool{}
	alive.Store(true)
	go func() {
		log := logger.Logger()
		for {
			time.Sleep(time.Second * 5)
			if err := db.Ping(); err != nil {
				log.With(zap.Error(err)).Error("fail to ping database, set alive=false")
				alive.Store(false)
			}
		}
	}()
	pingFn := func(writer http.ResponseWriter, request *http.Request) {
		if !alive.Load() {
			http.Error(writer, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		writer.WriteHeader(200)
		_, _ = writer.Write([]byte("ok"))
	}
	mux.HandleFunc("/readyz", pingFn)
	mux.HandleFunc("/healthz", pingFn)
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	return mux
}
