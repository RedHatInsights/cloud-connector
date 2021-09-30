package jwt_utils

import (
	"context"
	"crypto/rsa"
	"io/ioutil"
	"time"

	"github.com/RedHatInsights/cloud-connector/internal/platform/logger"

	"github.com/dgrijalva/jwt-go"
	"github.com/sirupsen/logrus"
)

type clientInfo struct {
	ClientID           string `json:"client-id"`
	AuthorizationGroup string `json:"auth-group"`
}
type customClaims struct {
	*jwt.StandardClaims
	clientInfo
}

const (
	RsaTokenGenerator  = "jwt_rsa_generator"
	FileTokenGenerator = "jwt_file_reader"
)

func createRsaToken(client string, group string, exp time.Time, signKey *rsa.PrivateKey) (string, error) {
	t := jwt.New(jwt.GetSigningMethod("RS256"))
	t.Claims = &customClaims{
		&jwt.StandardClaims{
			ExpiresAt: exp.UTC().Unix(),
		},
		clientInfo{client, group},
	}
	t.Header["kid"] = "rhcloud-connector"
	return t.SignedString(signKey)
}

type JwtGenerator func(c context.Context) (string, error)

func NewFileBasedJwtGenerator(jwtFilename string) (JwtGenerator, error) {
	filename := jwtFilename
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

func NewRSABasedJwtGenerator(privateKeyFile string, clientId string, tokenExpiry int) (JwtGenerator, error) {
	signBytes, err := ioutil.ReadFile(privateKeyFile)
	if err != nil {
		return nil, err
	}
	signKey, err := jwt.ParseRSAPrivateKeyFromPEM(signBytes)
	if err != nil {
		return nil, err
	}
	return func(context context.Context) (string, error) {
		expiryDate := time.Now().Add(time.Minute * time.Duration(tokenExpiry))
		logger.Log.Debugf("Generating an RSA JWT token with client-id %s and expiry: %s\n", clientId, expiryDate)
		return createRsaToken(clientId, "admin", expiryDate, signKey)
	}, nil
}
