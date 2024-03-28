#for i in $(seq 1 10);
#do
#    echo $i
#    sleep 4
#done
#
#
#sleep 1
#echo "blah..."
#echo "blah..."
#echo "CONNECTED fred-1234"
#sleep 2

#go run simple_test_client.go -broker ssl://localhost:8883 -cert dev/test_client/client-0-cert.pem -key dev/test_client/client-0-key.pem

CERT=HARPERDB/192.168.131.16/cert.pem
KEY=HARPERDB/192.168.131.16/key.pem
BROKER="wss://rh-gtm.harperdbcloud.com:443"

BROKER="ssl://localhost:8883" 
CERT="dev/test_client/client-0-cert.pem"
KEY="dev/test_client/client-0-key.pem"

#go run simple_test_client.go -broker $BROKER -cert $CERT -key $KEY

BROKER="tcp://mosquitto:1883"
CERT="/tmp/tls/cert.pem"
KEY="/tmp/tls/key.pem"

echo "\$BROKER=$BROKER"
echo "\$CERT=$CERT"
echo "\$KEY=$KEY"

./load_tester mqtt_client --broker=$BROKER --cert-file=$CERT --key-file=$KEY
