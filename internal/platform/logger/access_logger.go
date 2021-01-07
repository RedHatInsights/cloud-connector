package logger

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/gorilla/handlers"
	"github.com/sirupsen/logrus"
)

func AccessLoggerMiddleware(next http.Handler) http.Handler {
	return handlers.CustomLoggingHandler(ioutil.Discard, next, logrusAccessLogAdapter)
}

// This is a bit of a hack as well.  This method should be writing to the io.Writer.
// Unfortunately, it doesn't seem possible to use the logrus Fields when writing
// to the io.Writer object directly.  It might be cleaner to implement our own
// logging handler eventually.
func logrusAccessLogAdapter(w io.Writer, params handlers.LogFormatterParams) {
	request := fmt.Sprintf("%s %s %s", params.Request.Method, params.Request.URL, params.Request.Proto)
	requestID := params.Request.Header.Get("X-Rh-Insights-Request-Id")
	Log.WithFields(logrus.Fields{
		"remote_addr": params.Request.RemoteAddr,
		"request":     request,
		"request_id":  requestID,
		"status":      params.StatusCode,
		"size":        params.Size},
	).Info("access")
}
