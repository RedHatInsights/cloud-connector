package logger

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	lc "github.com/redhatinsights/platform-go-middlewares/logging/cloudwatch"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Log is an instance of the global logrus.Logger
var Log *logrus.Logger
var logLevel logrus.Level
var initializeLogger sync.Once

func buildFormatter(format string) logrus.Formatter {
	switch strings.ToUpper(format) {
	case "TEXT":
		return &logrus.TextFormatter{}
	default:
		return NewCloudwatchFormatter()
	}
}

// NewCloudwatchFormatter creates a new log formatter
func NewCloudwatchFormatter() *CustomCloudwatch {
	f := &CustomCloudwatch{}

	var err error
	if f.Hostname == "" {
		if f.Hostname, err = os.Hostname(); err != nil {
			f.Hostname = "unknown"
		}
	}

	return f
}

//Format is the log formatter for the entry
func (f *CustomCloudwatch) Format(entry *logrus.Entry) ([]byte, error) {
	b := &bytes.Buffer{}

	now := time.Now()

	hostname, err := os.Hostname()
	if err == nil {
		f.Hostname = hostname
	}

	data := map[string]interface{}{
		"@timestamp":  now.Format("2006-01-02T15:04:05.999Z"),
		"@version":    1,
		"message":     entry.Message,
		"levelname":   entry.Level.String(),
		"source_host": f.Hostname,
		"app":         "cloud-connector",
		"caller":      entry.Caller.Func.Name(),
	}

	for k, v := range entry.Data {
		switch v := v.(type) {
		case error:
			data[k] = v.Error()
		case Marshaler:
			data[k] = v.MarshalLog()
		default:
			data[k] = v
		}
	}

	j, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	b.Write(j)

	return b.Bytes(), nil
}

// InitLogger initializes the logger instance
func InitLogger() {

	initializeLogger.Do(func() {
		hostname, _ := os.Hostname()

		logconfig := viper.New()
		logconfig.SetDefault("LOG_LEVEL", "DEBUG")
		logconfig.SetDefault("LOG_GROUP", "platform-dev")
		logconfig.SetDefault("AWS_REGION", "us-east-1")
		logconfig.SetDefault("LOG_STREAM", hostname)
		logconfig.SetDefault("LOG_FORMAT", "text")
		logconfig.SetEnvPrefix("CLOUD_CONNECTOR")
		logconfig.AutomaticEnv()
		key := logconfig.GetString("CW_AWS_ACCESS_KEY_ID")
		secret := logconfig.GetString("CW_AWS_SECRET_ACCESS_KEY")
		region := logconfig.GetString("AWS_REGION")
		group := logconfig.GetString("LOG_GROUP")
		stream := logconfig.GetString("LOG_STREAM")
		format := logconfig.GetString("LOG_FORMAT")

		switch strings.ToUpper(logconfig.GetString("LOG_LEVEL")) {
		case "TRACE":
			logLevel = logrus.TraceLevel
		case "DEBUG":
			logLevel = logrus.DebugLevel
		case "ERROR":
			logLevel = logrus.ErrorLevel
		default:
			logLevel = logrus.InfoLevel
		}
		if flag.Lookup("test.v") != nil {
			logLevel = logrus.FatalLevel
		}

		formatter := buildFormatter(format)

		Log = &logrus.Logger{
			Out:          os.Stdout,
			Level:        logLevel,
			Formatter:    formatter,
			Hooks:        make(logrus.LevelHooks),
			ReportCaller: true,
		}

		if key != "" {
			Log.Infof("Configuring CloudWatch logging (level=%s, group=%s, stream=%s)",
				logLevel, group, stream)
			cred := credentials.NewStaticCredentials(key, secret, "")
			awsconf := aws.NewConfig().WithRegion(region).WithCredentials(cred)
			hook, err := lc.NewBatchingHook(group, stream, awsconf, 10*time.Second)
			if err != nil {
				Log.WithFields(logrus.Fields{"error": err}).Warn("Unable to configure CloudWatch hook")
			}
			Log.Hooks.Add(hook)
		}
	})
}
