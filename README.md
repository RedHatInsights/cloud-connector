# Cloud Connector

The Cloud Connector service is designed to receive messages from internal
clients and route the messages to the target machine which runs in
the customer's environment.

## Design

Cloud-Connector consists of 3 main components:

- MQTT message consumer - responsible for subscribing to the MQTT topics and putting the MQTT messages onto the kafka message queue
- Kafka message consumer - responsible for consuming the MQTT messages off of the kafka queue and recording the connection state in the database
- API server - responsible for passing messages from internal clients to the connected client via the MQTT broker

![Architecture diagram](/design/architecture.png)

### MQTT Broker ###

Cloud-Connector expects the MQTT broker to be configured to require mutually authenticated TLS.
The broker should use the CN from the subject of the cert as the MQTT identity/username.
The client will use the CN from the subject of the cert as the client-id while connecting.
This client-id will be used as a part of the topics for each client
(see the [Topics](#topics) section).
The broker should be configured to only allow the clients to publish and subscribe to their
topics to protect against client crosstalk.

### MQTT Message Consumer ###

The MQTT message consumer's main responsiblity is reading messages from the MQTT topics and
writing those messages to kafka.
The MQTT message consumer does as little processing of the message as possible.  Its responsibility is
to read the messages from the MQTT broker as quickly as possible and to write those messages to kafka
as quickly as possible.

The MQTT message consumer subscribes to the client's _/out_ topics using a wildcard instead of an explicit
client-id.  This allows the MQTT message consumer to receive messages from all of the clients.

When the MQTT message consumer writes message to kafka, it uses the MQTT client-id (which is read from the topic
from the MQTT message's metadata) is used as the _Key_ for the kafka message.  The client id is used as a message
key in order to main the ordering of the messages on the kafka message queue as all messages with the same key get
sent to the same kafka partitions.

It is important to note that the MQTT message consumer must 1) use the same client-id when connecting
to the MQTT broker and 2) set the MQTT _cleanSession_ flag to _false_ when connecting to the MQTT broker.
These 2 settings allow the broker to buffer messages intended for the Cloud-Connector MQTT consumer while
the MQTT consumer is down.

### Kafka Message Consumer ###

The Kafka message consumer reads the messages from the kafka message queue and processes those messages.
The messages contain the state of the client connection (mainly either _online_ or _offline_).  The kafka
message consumer updates the connection state in the database based on the message content.

The Kafka message consumer is only an MQTT publisher.  The Kafka message uses the pod name as the MQTT client-id.

### API Server ###

The API server is responsible for allowing internal clients to pass messages to the connected clients via
the MQTT broker.  When the API server receives a message, it looks up the connection details in the connection 
database.  If the connection is _online_, then the API server sends the message to the MQTT broker for delivery.

The API server is only an MQTT publisher.  The API server uses the pod name as the MQTT client-id.

## Protocol

### Topics ###

For every *Client*, a pair of topics exist to enable the *Client* and the
*Server* to communicate with each other. A second pair of topics exist that
enable instructions to be dispatched to worker processes on the client.

#### Control Topics ####

| **Topic**                              | **Client** | **Server** |
| -------------------------------------- | ---------- | ---------- |
| `${prefix}/${client_uuid}/control/in`  | subscribe  | publish    |
| `${prefix}/${client_uuid}/control/out` | publish    | subscribe  |

The *Client* subscribes to the `${prefix}/${client_uuid}/control/in`
topic. Any messages it receives on that topic that by definition require a
reply are published on the `${prefix}/${client_uuid}/control/out` topic.
Conversely, the *Server* subscribes to the `${prefix}/${client_uuid}/control/out`
topic. Any messages it receives on that topic that by definition require a
reply are published on the `${prefix}/${client_uuid}/control/in` topic.

Either the *Client* or the *Server* may initiate a message to the other by
publishing a message on its respective "publish" topic.

#### Payload Topics ####

As the _Control Topics_ are meant as a means by which the *Client* and
*Server* can communicate with each other, the _Payload Topics_ are a vector
by which the *Server* can dispatch messages destined for worker processes of
the *Client*.

| **Topic**                           | **Client** | **Server** |
| ------------------------------------| ---------- | ---------- |
| `${prefix}/${client_uuid}/data/in`  | subscribe  | publish    |
| `${prefix}/${client_uuid}/data/out` | publish    | subscribe  |

### Messages ###

All MQTT messages contain JSON as their payload.

#### Control Messages ####

All control messages include the follow fields as an "envelope". Any
message-specific fields should be included in the `content` object.

| **Field**     | **Type**         | **Optional** | **Example**                              |
| ------------- | ---------------- | ------------ | ---------------------------------------- |
| `type`        | string           | no           | `"client-handshake"`                     |
| `message_id`  | string(uuid)     | no           | `"b5953dee-5e91-4f88-8cdc-962cbe290cdc"` |
| `response_to` | string(uuid)     | yes          | `"7260552e-81ed-49db-a6fe-290929dfccf9"` |
| `version`     | integer          | no           | `1`                                      |
| `sent`        | string(ISO-8601) | no           | `"2021-01-12T14:58:13+00:00"`            |
| `content`     | object           | yes          | `{}`                                     |

##### Connection Status #####

A `ConnectionStatus` message is initiated by the *Client*. It is published as a
QOS 1 message when the client initializes itself at startup. When a client is
shutting down, it will publish a `ConnectionStatus` message with the state
set to `offline`.  The `offline` message will be published as a QOS 1 message.

The `content` field of a `ConnectionStatus` message must contain two fields:

| **Field**         | **Type**     | **Optional** |
| ----------------- | ------------ | ------------ |
| `canonical_facts` | object       | no           |
| `dispatchers`     | object       | no           |
| `state`           | string(enum) | no           |
| `tags`            | object       | yes          |
| `client_name      | string       | yes          |
| `client_version`  | string       | yes          |


`state` must be one of the following values:

| **State** |
| --------- |
| `online`  |
| `offline` |

The `canonical_facts` object must include the following fields:

| **Field**                 | **Type**      | **Optional** | **Example**                                 |
| ------------------------- | ------------- | ------------ | ------------------------------------------- |
| `insights_id`             | string(uuid)  | no           | `"3a57b1ad-5163-47ee-9e57-3bb6d90bdfff"`    |
| `machine_id`              | string(uuid)  | no           | `"3fb0c7be-89cb-4f19-84b7-94448f40f769"`    |
| `bios_uuid`               | string(uuid)  | no           | `"3fb0c7be-89cb-4f19-84b7-94448f40f769"`    |
| `subscription_manager_id` | string(uuid)  | no           | `"63a12856-a262-4e1e-b562-c099a735ca76"`    |
| `ip_addresses`            | array(string) | no           | `["192.168.122.162"]`                       |
| `mac_addresses`           | array(string) | no           | `["52:54:00:66:ea:9a","00:00:00:00:00:00"]` |
| `fqdn`                    | string        | no           | `"ic-rhel8-dev-thelio"`                     |

The `dispatchers` object includes any number of objects where the field name is
the routable name of the dispatch destination, and the value is an object of
arbitrary key-value pairs, as reported by the worker process.

The `tags` object includes an arbitrary number of key-value pairs where the
key and value are both strings, as reported by the client. The field is
optional, so it may be omitted.

The optional `client_version` field contains the software version as reported by
the client.

A complete example of a `ConnectionStatus` message:

```
{
    "type": "connection-status",
    "message_id": "3a57b1ad-5163-47ee-9e57-3bb6d90bdfff",
    "version": 1,
    "sent": "2020-12-04T17:22:24+00:00",
    "content": {
        "canonical_facts": {
            "insights_id": "cabb61b6-e61d-4d70-b475-01f5c009e93c",
            "machine_id": "3fb0c7be-89cb-4f19-84b7-94448f40f769",
            "bios_uuid": "3fb0c7be-89cb-4f19-84b7-94448f40f769",
            "subscription_manager_id": "63a12856-a262-4e1e-b562-c099a735ca76",
            "ip_addresses": ["192.168.122.162"],
            "mac_addresses": ["52:54:00:66:ea:9a","00:00:00:00:00:00"],
            "fqdn": "ic-rhel8-dev-thelio"
        },
        "dispatchers": {
            "playbook": {
                "ansible-runner-version": "1.2.3"
            },
            "echo": {}
        },
        "state": "online",
        "tags": {
            "region": "us1",
            "prod": "false"
        },
        "client_name": "yggdrasil"
        "client_version": "0.1.4"
    }
}
```

A complete example of an offline `ConnectionStatus` message:

```
{
    "type": "connection-status",
    "message_id": "3a57b1ad-5163-47ee-9e57-3bb6d90bdfff",
    "version": 1,
    "sent": "2020-12-04T17:22:24+00:00",
    "content": {
        "state": "offline"
    }
}
```

##### Command #####

A `Command` message is initiated by the *Server*. It is published when the
server needs to order a *Client* to perform a specific operation. No reply is
expected.

| **Field**   | **Type**     | **Optional** | **Example**   |
| ----------- | ------------ | ------------ | ------------- |
| `command`   | string(enum) | no           | `"reconnect"` |
| `arguments` | object       | yes          | `{}`          |
| `message`   | string       | yes          | `"authentication failure"`|

`command` must be one of the following values:

| **Command**  | **Arguments**  |
| ------------ | -------------- |
| `reconnect`  | `{"delay": 5}` |
| `ping`       |                |
| `disconnect` |                |

A complete example of a `Command` message:

```
{
    "type": "command",
    "message_id": "3a57b1ad-5163-47ee-9e57-3bb6d90bdfff",
    "version": 1,
    "sent": "2020-12-04T17:22:24+00:00",
    "content": {
        "command": "reconnect",
        "arguments": {
            "delay": 5
        },
        "message": "authentication failure"
    }
}
```

##### Event #####

An `Event` message is initiated by the *Client*. It is published when prescribed
events are occurring in the *Client*. No reply is expected.

`content` must be one of the following values:

| **Event**    |
| ------------ |
| `disconnect` |
| `pong`       |

A complete example of an `Event` message:

```
{
    "type": "event",
    "message_id": "3a57b1ad-5163-47ee-9e57-3bb6d90bdfff",
    "response_to": "7260552e-81ed-49db-a6fe-290929dfccf9",
    "version": 1,
    "sent": "2020-12-04T17:22:24+00:00",
    "content": "disconnect"
}
```

#### Data Messages ####

All data messages include the follow fields as an "envelope". Any
message-specific fields should be included in the `content` object.

| **Field**       | **Type**         | **Optional** | **Example**                              |
| --------------- | ---------------- | ------------ | ---------------------------------------- |
| `type`          | string           | no           | `"payload-dispatch"`                     |
| `message_id`    | string(uuid)     | no           | `"b5953dee-5e91-4f88-8cdc-962cbe290cdc"` |
| `response_to`   | string(uuid)     | yes          | `"b6e219d2-44a5-4d9b-a7da-57f1aacefcf3"` |
| `version`       | integer          | no           | `1`                                      |
| `sent`          | string(ISO-8601) | no           | `"2021-01-12T14:58:13+00:00"`            |
| `directive`     | string           | no           | `"playbook"`                             |
| `metadata`      | object           | yes          | `{}`                                     |
| `content`       |                  | yes          | `{}`                                     |

##### Data #####

A `Data` message is initiated by either the *Client* or the *Server*. The
*Server* publishes it when data needs to be delivered to a worker on the
*Client*. The *Client* publishes a `Data` message when it needs to report about
a previously received `Data` message.

A complete example of a data message as published by the *Server*:

```
{
    "type": "data",
    "message_id": "a6a7d866-7de0-409a-84e0-3c56c4171bb7",
    "version": 1,
    "sent": "2021-01-12T15:30:08+00:00",
    "directive": "playbook",
    "metadata": {
        "return_url": "https://cloud.redhat.com/api/v1/ingres/upload"
    },
    "content": "https://cloud.redhat.com/api/v1/remediations/1234/playbook"
}
```

A second complete example of a data message as published by the *Server*:
```
{
    "type": "data",
    "message_id": "a6a7d866-7de0-409a-84e0-3c56c4171bb7",
    "version": 1,
    "sent": "2021-01-12T15:30:08+00:00",
    "directive": "echo",
    "content": "Hello world!"
}
```

Two complete examples of data messages as published by the *Client*:
```
{
    "type": "data",
    "message_id": "a6a7d866-7de0-409a-84e0-3c56c4171bb7",
    "response_to": "b6e219d2-44a5-4d9b-a7da-57f1aacefcf3",
    "version": 1,
    "sent": "2021-01-12T15:30:08+00:00",
    "directive": "",
    "content": "accepted"
}
```

```
{
    "type": "data",
    "message_id": "a6a7d866-7de0-409a-84e0-3c56c4171bb7",
    "response_to": "b6e219d2-44a5-4d9b-a7da-57f1aacefcf3",
    "version": 1,
    "sent": "2021-01-12T15:30:08+00:00",
    "directive": "",
    "content": "rejected"
}
```

## Internal REST interface

The internal REST interface can be used to retreive the list of connections which belong to an organization,
get information about a connection and send messages to a connected client.
This REST interface is only available within the cluster where cloud-connector is deployed.

In order to authenticate, the client is required to pass either the identity header or the following
pre-shared key headers:

```
"x-rh-cloud-connector-org-id"
"x-rh-cloud-connector-client-id"
"x-rh-cloud-connector-psk"
```

If your service is internal and will not be passing requests through 3scale a psk will be provided.
This psk will be unique to your service.
The `x-rh-cloud-connector-org-id` is required so that cloud-connector knows which organzation it is
acting on behalf of.

### Get list of connections for an organization

```
GET /v2/connections
```

### Get status of a connection

```
GET /v2/connections/{client_id}/status
```

### Send a message to a connected client
```
POST /v2/connections/{client_id}/message
```

See [API schema](./internal/controller/api/api.spec.json) for more details.


## License

This project is available as open source under the terms of the [Apache License 2.0](http://www.apache.org/licenses/LICENSE-2.0).
