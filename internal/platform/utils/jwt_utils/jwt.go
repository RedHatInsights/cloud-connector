package jwt_utils

import (
	"context"
	"errors"
	"io/ioutil"

	"github.com/RedHatInsights/cloud-connector/internal/config"
	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	"github.com/sirupsen/logrus"
)

type JwtGenerator func(context.Context) (string, error)

func NewJwtGenerator(implName string, cfg *config.Config) (JwtGenerator, error) {

	switch implName {
	case "jwt_file_reader":
		return readJwtFromFile(cfg)
	default:
		return nil, errors.New("Invalid JwtGenerator impl requested")
	}
}

func readJwtFromFile(cfg *config.Config) (JwtGenerator, error) {
	filename := cfg.MqttBrokerJwtFile
	logger.Log.Debug("Loading JWT from a file: ", filename)

	jwtBytes, err := ioutil.ReadFile(filename)
	if err != nil {
		logger.Log.WithFields(logrus.Fields{"error": err}).Error("Could not read jwt from file")
		return nil, err
	}

	jwtText := string(jwtBytes)

	return func(context.Context) (string, error) {
		return jwtText, nil
	}, nil
}
