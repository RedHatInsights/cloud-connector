
# Port-forward
#BROKER="tcp://localhost:9883"

BROKER="ssl://mosquitto-route-connector-ci.5a9f.insights-dev.openshiftapps.com:443"
BROKER="ssl://localhost:8883"
BROKER="tcp://localhost:9883"

WORKING_DIR=/home/dehort/dev/go/src/github.com/RedHatInsights/cloud-connector
WORKING_DIR=./dev/test_client

./test_client -broker $BROKER -connection_count 1 -cert ${WORKING_DIR}/client-0-cert.pem -key ${WORKING_DIR}/client-0-key.pem


