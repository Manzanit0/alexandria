package httpx

import (
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/manzanit0/alexandria/tgram"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"golang.org/x/exp/slog"
)

// TraceRequests returns middleware that will trace incoming requests.
func TraceRequests(serviceName string) gin.HandlerFunc {
	return otelgin.Middleware(serviceName)
}

// LogRequests returns middleware that logs every incoming request.
func LogRequests() gin.HandlerFunc {
	return func(c *gin.Context) {
		t0 := time.Now()

		c.Next()

		// Ofuscate access_key if it exists.
		q := c.Request.URL.Query()
		if q.Has("access_key") {
			q.Set("access_key", "*****")
		}

		slog.Info("inbound request",
			"http.request.duration_ms", time.Since(t0).Milliseconds(),
			"http.request.method", c.Request.Method,
			"http.request.url.scheme", c.Request.URL.Scheme,
			"http.request.url.host", c.Request.URL.Host,
			"http.request.url.path", c.Request.URL.Path,
			"http.request.url.query_params", q,
			"http.request.content_length", c.Request.ContentLength,
			"http.request.headers", c.Request.Header)
	}
}

// Recovery returns middleware that recovers from panics and reports them to a Telegram channel.
func Recovery(t tgram.Client, reportChat int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				callstack := getCallstack()
				slog.Error(fmt.Sprint("recovered from panic", callstack))

				if t != nil {
					_ = t.SendMessage(tgram.SendMessageRequest{
						ParseMode: tgram.ParseModeHTML,
						ChatID:    reportChat,
						Text: fmt.Sprintf(`<b>Recovered from panic: %v</b>
<code>%s</code>`, r, callstack),
					})
				}

				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()

		c.Next()
	}
}

func getCallstack() string {
	pcs := make([]uintptr, 20)
	depth := runtime.Callers(3, pcs)
	frames := runtime.CallersFrames(pcs[:depth])

	var sb strings.Builder
	for f, more := frames.Next(); more; f, more = frames.Next() {
		sb.WriteString(fmt.Sprintf("%s: %d\n", f.Function, f.Line))
	}

	return sb.String()
}
