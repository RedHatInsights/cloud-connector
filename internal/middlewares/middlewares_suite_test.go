package middlewares_test

import (
	"testing"

	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMiddlewares(t *testing.T) {
	RegisterFailHandler(Fail)
	logger.InitLogger()
	RunSpecs(t, "Middlewares Suite")
}
