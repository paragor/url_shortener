package logger

import (
	"context"
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"net/http"
)

func HttpSetLoggerMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestId := r.Header.Get("X-Request-ID")
		ctx := r.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		logger := Logger()
		r = r.WithContext(ToCtx(logger.With(zap.String("request_id", requestId)), ctx))
		handler.ServeHTTP(w, r)
	})
}

func HttpRecoveryMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				FromCtx(r.Context()).
					WithOptions(zap.AddStacktrace(zapcore.ErrorLevel)).
					With(zap.Error(fmt.Errorf("%v", err))).
					Error("panic on request handler")
				w.WriteHeader(http.StatusInternalServerError)
			}
		}()

		handler.ServeHTTP(w, r)
	})
}
