package caching

import (
	"bytes"
	"github.com/labstack/echo/v4"
	"github.com/patrickmn/go-cache"
	"io"
	"net/http"
	"strings"
)

type cacheEntry struct {
	body     []byte
	mimeType string
}

type bodyDumpResponseWriter struct {
	io.Writer
	http.ResponseWriter
	statusCode int
}

func (w *bodyDumpResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *bodyDumpResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func (w *bodyDumpResponseWriter) Flush() {
	w.ResponseWriter.(http.Flusher).Flush()
}

func CacheMiddleware(avatarCache *cache.Cache) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(context echo.Context) error {
			if cachedAvatar, found := avatarCache.Get(context.Request().RequestURI); found {
				context.Response().Header().Add("Cache-Status", "HIT")
				return context.Blob(http.StatusOK, cachedAvatar.(cacheEntry).mimeType, cachedAvatar.(cacheEntry).body)
			}

			resBody := new(bytes.Buffer)
			mw := io.MultiWriter(context.Response().Writer, resBody)
			writer := &bodyDumpResponseWriter{Writer: mw, ResponseWriter: context.Response().Writer}
			context.Response().Writer = writer

			err := next(context)
			if err != nil {
				context.Error(err)
			}

			// Check if the response is valid, AND if the response has a provided seed (:name). Don't cache random generated ones
			contentType := context.Response().Header().Get("Content-Type")
			if writer.statusCode == http.StatusOK && (contentType == "image/png" || contentType == "image/svg+xml") && strings.Contains(context.Path(), ":name") {
				avatarCache.SetDefault(context.Request().RequestURI, cacheEntry{
					body:     resBody.Bytes(),
					mimeType: contentType,
				})
			}

			return nil
		}
	}
}
