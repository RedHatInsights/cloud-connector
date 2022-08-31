JOB_RECEIVER_HOST=localhost:8080
JOB_RECEIVER_HOST=localhost:10000
JOB_RECEIVER_HOST=localhost:8081



DIRECTIVE="        "
DIRECTIVE="echo"
PAYLOAD="Hello World!"


# QA
ACCOUNT="6212377"
NODE_ID="fabaaf88-0be0-402f-b3b1-21cd39731003"

ACCOUNT="6089719"
NODE_ID="9a52d3d9-9276-4c52-97a4-6689ab413f65"

ACCOUNT=477931
NODE_ID="16970932-39b7-409f-b703-506241a2f16a"


NODE_ID="8a823918-f801-474c-9f79-7cc7e9d79a7f"

ACCOUNT="000111"
ACCOUNT="111000"
ACCOUNT="010101"

AUTH_TYPE="basic"
AUTH_TYPE="Associate"
ORG_ID="10000"
ORG_ID="10001"
ORG_ID="0002"

echo "JOB_RECEIVER_HOST: $JOB_RECEIVER_HOST"
echo "AUTH_TYPE: $AUTH_TYPE"
echo "ACCOUNT: $ACCOUNT"
echo "ORG_ID: $ORG_ID"
echo "NODE_ID: $NODE_ID"

# x-rh-cloud-connector-account + x-rh-cloud-connector-client-id + x-rh-cloud-connector-psk)


JSON_IDENTITY_HEADER="{\"identity\": {\"account_number\": \"$ACCOUNT\", \"internal\": {\"org_id\": \"$ORG_ID\"}, \"type\": \"$AUTH_TYPE\"}}"
echo "JSON_IDENTITY_HEADER: $JSON_IDENTITY_HEADER"
IDENTITY_HEADER=$(echo -n $JSON_IDENTITY_HEADER | openssl base64 -A)
echo "IDENTITY_HEADER: $IDENTITY_HEADER"


function getConnectionsByAccount {
    echo ""
    echo "Connections By Account: $ACCOUNT"
    #curl -s -H "x-rh-identity: $IDENTITY_HEADER" "http://$JOB_RECEIVER_HOST/api/cloud-connector/v1/connection/$ACCOUNT" | jq
    curl -v -s -H "x-rh-identity: $IDENTITY_HEADER" "http://$JOB_RECEIVER_HOST/api/cloud-connector/v1/connection/$ACCOUNT?offset=0&limit=10" | jq
}

function getConnectionsByOrgIdV2 {
    echo ""
    echo "Connections By Org ID: $ORG_ID, Account: $ACCOUNT"
    #curl -s -H "x-rh-identity: $IDENTITY_HEADER" "http://$JOB_RECEIVER_HOST/api/cloud-connector/v2/connections/" | jq
    curl -v -s -H "x-rh-identity: $IDENTITY_HEADER" "http://$JOB_RECEIVER_HOST/api/cloud-connector/v2/connections?offset=0&limit=10" | jq
}

function getAllConnections {
    echo ""
    echo "Connections:"
    curl  -s -H "x-rh-identity: $IDENTITY_HEADER" "http://$JOB_RECEIVER_HOST/api/cloud-connector/v1/connection?offset=0&limit=1000" | jq
    #curl -s -H "x-rh-identity: $IDENTITY_HEADER" "http://$JOB_RECEIVER_HOST/api/cloud-connector/v1/connection?offset=2&limit=5" | jq
    #curl -s -H "x-rh-identity: $IDENTITY_HEADER" "http://$JOB_RECEIVER_HOST/api/cloud-connector/v1/connection?limit=10" | jq
}


function sendMessage {
    echo ""
    echo "Sending a message to $ACCOUNT : $NODE_ID - directive: $DIRECTIVE"
    PAYLOAD=""
    curl -v -X POST -d "{\"account\": \"$ACCOUNT\", \"recipient\": \"$NODE_ID\", \"directive\": \"$DIRECTIVE\", \"payload\": \"$PAYLOAD\"}" -H "x-rh-identity: $IDENTITY_HEADER" -H "x-rh-insights-request-id: 1234" http://$JOB_RECEIVER_HOST/api/cloud-connector/v1/message
}


function connection_status {
    echo ""
    echo "Status: $ACCOUNT - $NODE_ID"
    curl -s -X POST -d "{\"account\": \"$ACCOUNT\", \"node_id\": \"$NODE_ID\"}" -H  "x-rh-identity: $IDENTITY_HEADER" -H "x-rh-insights-request-id: testing1234" http://$JOB_RECEIVER_HOST/api/cloud-connector/v1/connection_status | jq
}


function ping {
    echo ""
    echo "Ping: $ACCOUNT - $NODE_ID"
    curl -s -X POST -d "{\"account\": \"$ACCOUNT\", \"node_id\": \"$NODE_ID\"}" -H  "x-rh-identity: $IDENTITY_HEADER" -H "x-rh-insights-request-id: testing1234" http://$JOB_RECEIVER_HOST/api/cloud-connector/v1/connection/ping | jq
}


function disconnect {
    echo ""
    echo "Disconnect: $ACCOUNT - $NODE_ID"
    curl -s -X POST -d "{\"account\": \"$ACCOUNT\", \"node_id\": \"$NODE_ID\", \"message\": \"stop it now!\"}" -H  "x-rh-identity: $IDENTITY_HEADER" -H "x-rh-insights-request-id: testing1234" http://$JOB_RECEIVER_HOST/api/cloud-connector/v1/connection/disconnect | jq
}


function reconnect {
    echo ""
    echo "Reconnect: $ACCOUNT - $NODE_ID"
    curl -v -s -X POST -d "{\"account\": \"$ACCOUNT\", \"node_id\": \"$NODE_ID\", \"delay\": 40, \"message\": \"something bad happened\"}" -H  "x-rh-identity: $IDENTITY_HEADER" -H "x-rh-insights-request-id: testing1234" http://$JOB_RECEIVER_HOST/api/cloud-connector/v1/connection/reconnect | jq
}


function status {
    echo ""
    echo "Status: $ACCOUNT - $NODE_ID"
    curl -s -X POST -d "{\"account\": \"$ACCOUNT\", \"node_id\": \"$NODE_ID\"}" -H  "x-rh-identity: $IDENTITY_HEADER" -H "x-rh-insights-request-id: testing1234" http://$JOB_RECEIVER_HOST/api/cloud-connector/v1/connection/status | jq
}


function reenable {
    echo ""
    echo "Re-enable: $ACCOUNT - $NODE_ID"
    curl -s -X POST -d "{\"account\": \"$ACCOUNT\", \"node_id\": \"$NODE_ID\"}" -H  "x-rh-identity: $IDENTITY_HEADER" -H "x-rh-insights-request-id: testing1234" http://$JOB_RECEIVER_HOST/api/cloud-connector/v1/connection/reenable | jq
}


function pskTest {
    PSK_ID="fred"
    PSK_KEY="fredskey"

    #curl -v -X POST -d "{\"account\": \"$ACCOUNT\", \"recipient\": \"$NODE_ID\", \"directive\": \"$DIRECTIVE\", \"payload\": \"$PAYLOAD\"}" -H "x-rh-cloud-connector-account: $ACCOUNT" -H "x-rh-cloud-connector-client-id: $PSK_ID" -H "x-rh-cloud-connector-psk: $PSK_KEY" -H $REQUEST_ID_HEADER -H "x-rh-insights-request-id: 1234" http://$JOB_RECEIVER_HOST/job


    #curl -v -s -X POST -d "{\"account\": \"$ACCOUNT\", \"node_id\": \"$NODE_ID\"}" -H "x-rh-cloud-connector-account: $ACCOUNT" -H "x-rh-cloud-connector-client-id: $PSK_ID" -H "x-rh-cloud-connector-psk: $PSK_KEY" -H $REQUEST_ID_HEADER -H "x-rh-insights-request-id: 1234" "http://$JOB_RECEIVER_HOST/api/cloud-connector/v1/connection/status" | jq

    curl -v -s -H "x-rh-cloud-connector-account: $ACCOUNT" -H "x-rh-cloud-connector-client-id: $PSK_ID" -H "x-rh-cloud-connector-psk: $PSK_KEY" -H $REQUEST_ID_HEADER -H "x-rh-insights-request-id: 1234" "http://$JOB_RECEIVER_HOST/api/cloud-connector/v1/connection/$ACCOUNT" | jq
}

function permitted_tenant_connection_status {

    #ACCOUNT="010101"
    #ACCOUNT="000000"
    #ORG_ID=""
    #ORG_ID="0002"
    PSK_ID="fred"
    PSK_KEY="fredskey"

    echo ""
    echo "Status: $ACCOUNT - $NODE_ID"

    #WONKY_ACCOUNT="010101"
    WONKY_ACCOUNT=$ACCOUNT

    echo "WONKY_ACCOUNT = $WONKY_ACCOUNT"

    #curl -v -s -X POST -d "{\"account\": \"$WONKY_ACCOUNT\", \"node_id\": \"$NODE_ID\"}" \
    #-H "x-rh-identity: $IDENTITY_HEADER" \
    #-H $REQUEST_ID_HEADER \
    #-H "x-rh-insights-request-id: 1234" \
    #"http://$JOB_RECEIVER_HOST/api/cloud-connector/v1/connection_status" | jq


    #-H "x-rh-cloud-connector-org-id: $ORG_ID" \

    curl -s -X POST -d "{\"account\": \"$ACCOUNT\", \"node_id\": \"$NODE_ID\"}" \
    -H "x-rh-cloud-connector-account: $ACCOUNT" \
    -H "x-rh-cloud-connector-client-id: $PSK_ID" \
    -H "x-rh-cloud-connector-psk: $PSK_KEY" \
    -H "x-rh-insights-request-id: 1234" \
    "http://$JOB_RECEIVER_HOST/api/cloud-connector/v1/connection_status" | jq
}

function permitted_tenant_send_message {
    echo ""
    echo "Sending a message to $ACCOUNT : $NODE_ID - directive: $DIRECTIVE"

    #curl -v -X POST \
    #-d "{\"account\": \"$ACCOUNT\", \"recipient\": \"$NODE_ID\", \"directive\": \"$DIRECTIVE\", \"payload\": \"$PAYLOAD\"}" \
    #-H "x-rh-identity: $IDENTITY_HEADER" \
    #-H "x-rh-insights-request-id: 1234" \
    #http://$JOB_RECEIVER_HOST/api/cloud-connector/v1/message

    #ACCOUNT="010101"
    ACCOUNT="000000"
    #ORG_ID="0002"
    PSK_ID="fred"
    PSK_KEY="fredskey"

    #-H "x-rh-cloud-connector-org-id: $ORG_ID" \

    curl -v -X POST -d "{\"account\": \"$ACCOUNT\", \"recipient\": \"$NODE_ID\", \"directive\": \"$DIRECTIVE\", \"payload\": \"$PAYLOAD\"}" \
    -H "x-rh-cloud-connector-account: $ACCOUNT" \
    -H "x-rh-cloud-connector-client-id: $PSK_ID" \
    -H "x-rh-cloud-connector-psk: $PSK_KEY" \
    -H "x-rh-insights-request-id: 1234" \
    http://$JOB_RECEIVER_HOST/api/cloud-connector/v1/message
}


## V2 - rh id

function connection_status_v2 {
    echo ""
    echo "Status: $ORG_ID - $NODE_ID"
    #curl -v -s -X GET -H  "x-rh-identity: $IDENTITY_HEADER" -H "x-rh-insights-request-id: testing1234" http://$JOB_RECEIVER_HOST/api/cloud-connector/v2/connections/$NODE_ID/status | jq


    #ACCOUNT="010101"
    #ACCOUNT="000000"
    #ORG_ID="0002"
    ORG_ID="10001"
    PSK_ID="fred"
    PSK_KEY="fredskey"


    curl -v -s -X GET \
    -H "x-rh-cloud-connector-org-id: $ORG_ID" \
    -H "x-rh-cloud-connector-client-id: $PSK_ID" \
    -H "x-rh-cloud-connector-psk: $PSK_KEY" \
    -H "x-rh-insights-request-id: 1234" \
    http://$JOB_RECEIVER_HOST/api/cloud-connector/v2/connections/$NODE_ID/status | jq
}

function send_message_v2 {
    echo ""
    echo "Sending a message to $ORG_ID: $NODE_ID - directive: $DIRECTIVE"

    #curl -v -X POST \
    #-d "{\"directive\": \"$DIRECTIVE\", \"payload\": \"$PAYLOAD\"}" \
    #-H "x-rh-identity: $IDENTITY_HEADER" \
    #-H "x-rh-insights-request-id: 1234" \
    #http://$JOB_RECEIVER_HOST/api/cloud-connector/v2/connections/$NODE_ID/message


    #ACCOUNT="010101"
    #ACCOUNT="000000"
    ORG_ID="0002"
    #ORG_ID="10001"
    PSK_ID="fred"
    PSK_KEY="fredskey"

    curl -s -v -X POST -d "{\"directive\": \"$DIRECTIVE\", \"payload\": \"$PAYLOAD\"}" \
    -H "x-rh-cloud-connector-account: $ACCOUNT" \
    -H "x-rh-cloud-connector-org-id: $ORG_ID" \
    -H "x-rh-cloud-connector-client-id: $PSK_ID" \
    -H "x-rh-cloud-connector-psk: $PSK_KEY" \
    -H "x-rh-insights-request-id: 1234" \
    http://$JOB_RECEIVER_HOST/api/cloud-connector/v2/connections/$NODE_ID/message | jq
}

function get_connections_by_org_id_v2 {
    echo ""
    echo "Connections By Org ID: $ORG_ID"
    curl -v -s -X GET -H  "x-rh-identity: $IDENTITY_HEADER" -H "x-rh-insights-request-id: testing1234" http://$JOB_RECEIVER_HOST/api/cloud-connector/v2/connections | jq
    #curl -v -s -H "x-rh-identity: $IDENTITY_HEADER" "http://$JOB_RECEIVER_HOST/api/cloud-connector/v1/connection/$ACCOUNT?offset=0&limit=10" | jq

    #ACCOUNT="010101"
    #ACCOUNT="000000"
    #ORG_ID="0002"
    PSK_ID="fred"
    PSK_KEY="fredskey"

    curl -v -s -X GET \
    -H "x-rh-cloud-connector-org-id: $ORG_ID" \
    -H "x-rh-cloud-connector-client-id: $PSK_ID" \
    -H "x-rh-cloud-connector-psk: $PSK_KEY" \
    -H "x-rh-insights-request-id: 1234" \
    http://$JOB_RECEIVER_HOST/api/cloud-connector/v2/connections | jq
}




#getConnectionsByAccount
#getAllConnections


#permitted_tenant_connection_status
#permitted_tenant_send_message


#getConnectionsByOrgIdV2

#ping
#sendMessage
#reconnect
status
#disconnect
#reenable
#pskTest


#connection_status_v2
#send_message_v2
#get_connections_by_org_id_v2
