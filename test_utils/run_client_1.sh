BROKER="ssl://localhost:8883"

WORKING_DIR=/home/dehort/dev/go/src/github.com/RedHatInsights/cloud-connector
WORKING_DIR=dev/test_client

./test_client -broker $BROKER -connection_count 1 -cert ${WORKING_DIR}/client-1-cert.pem -key ${WORKING_DIR}/client-1-key.pem


