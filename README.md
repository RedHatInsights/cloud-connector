- do we need a ping that is above the MQTT message but below and application level ping?
  - No ping needed at this point
- what about different topics for different "types" of connections?  Would that help with the load??
- change "in" topic to "status"??
  - only watch "status" for connection state
  - watch "in" later on for bi-directional communication


# Cloud Connector

The Cloud Connector service is designed to receive messages from internal
clients and route the messages to the target machine which runs in
the customer's environment.

## Protocol

Each connected client will have unique topics for publishing and subscribing. 
Actual topic string subject to change.

Connected-Client topics:
Subscribe: redhat/insights/out/$clientID
Publish: redhat/insights/in/$clientID


### Connection registration

A handshake message will need to be published in order to register a
connection with the cloud-connector.  The handshake message will be different 
if the connection is for a single host or for a proxy connection.

_The handshake message must be a retained message._

#### Host connection registration

```
{
    "type": "host-handshake",
    "message_id": "3a57b1ad-5163-47ee-9e57-3bb6d90bdfff",
    "version": 1,
    "sent": "2020-12-04T17:22:24+00:00",
    "payload": {
        "canonical_facts": {
            "insights_id": "cabb61b6-e61d-4d70-b475-01f5c009e93c",
            "machine_id": "3fb0c7be-89cb-4f19-84b7-94448f40f769",
            "bios_uuid": "3fb0c7be-89cb-4f19-84b7-94448f40f769",
            "subscription_manager_id": "63a12856-a262-4e1e-b562-c099a735ca76",
            "ip_addresses": ["192.168.122.162"],
            "mac_addresses": ["52:54:00:66:ea:9a","00:00:00:00:00:00"],
            "fqdn": "ic-rhel8-dev-thelio"
        }
    }
}

```

#### Proxy connection registration

```
{
    "type": "proxy-handshake",
    "message_id": "3a57b1ad-5163-47ee-9e57-3bb6d90bdfff",
    "version": 1,
    "sent": "2020-12-04T17:22:24+00:00",
    "payload": {
        "catalog_service_facts": {
          "source_type": "<string>",
          "application_type": "<string>"
        }
    }
}
```

### Connection registration response

FIXME:  Should this be a type="handshake-response" ??

#### Success

FIXME:  Do not send anything on success?  This makes it easier in the case of processing 
the retained handshake messages.  But it makes it difficult for a client to know when 
to start processing messages.

```
```

#### Failure

If an handshake-error message is received, then the client should disconnect
and send a new handshake message at a later time.

# FIXME:  This could happen asynchronously due to the retained message processing with a new consumer.

```
{
    "type": "handshake-error",
    "message_id": "xxx-xx-xxx",
    "in_response_to": "3a57b1ad-5163-47ee-9e57-3bb6d90bdfff",
    "version": 1,
    "sent": "2020-12-04T17:22:24+00:00",
    "payload": {
       "details": "error detail go here"
    }
}
```

### Connection deregistration

Cloud-Connector needs to know when a client disconnects.  The disconnect can happen
when the Cloud-Connector is running or not running.

#### Clean/routine disconnection

I am currently thinking the "handshake" message needs to be a retained message.  This will
allow new connections to be recorded even while the cloud-connector is not running.

We also have to be able to record disconnection events.  The disconnection events can happen
when the cloud-connector is running and when it is not running.

I think the disconnect events can be handled in 2 ways:
1)
 - Send an "offline" message (not a retained message)
   - this will tell a running cloud-connector that the connection closed
 - Remove retained handshake message
   - this way a freshly restarted cloud-connector will not know about the previously established connection
 - this approach should lower the number of retained messages...which should lower
   the retained message processing required when the cloud-connector is restarted
2) 
 - Send a retained "offline" message
 - this approach will mean that each connected and disconnected client will have a
   retained connection status message that will need to be processed when the
   cloud-connector is restarted

#### Abnormal disconnection

Send a retained disconnect message

```
{
    "type": "offline",
    "message_id": "33390934-7628-49f6-88ea-528ef740c774",
    "version": 1
    "sent": "2020-12-04T17:22:24+00:00"
}

```


### Message

```
{
    "type": "message",
    "message_id": "33390934-7628-49f6-88ea-528ef740c774",
    "version": 1,
    "sent": "2020-12-04T17:19:47+00:00",
    "directive": "<user defined string>",  // rhc:work, remediations:fifi, catalog:ping
    "payload": {
      <user defined payload>
    }
}
```

The directive and payload are defined by the application.  The Cloud-Connector will accept the 
directive and payload from a REST call and pass that data to the connected client.

What if client cannot handle the message?  Can't parse it, can't dispatch it, etc?
Send back a message-response of type error?

```
{
    "type": "message-error",
    "message_id": "xxx-xx-xxx",
    "in_response_to": "33390934-7628-49f6-88ea-528ef740c774",
    "version": 1,
    "sent": "2020-12-04T17:19:47+00:00",
    "payload": {
      "details": "error details"
    }
}
```


### Ping operation

No, ping for now.

### Force disconnect

The Cloud-Connector can send a force-disconnect message to the client.   Upon receiving 
the force-disconnect message, the client should disconnect (see the disconnect messages above) 
and reconnect.

```
{
    "type": "force-disconnect",
    "message_id": "33390934-7628-49f6-88ea-528ef740c774",
    "version": 1,
    "sent": "2020-12-04T17:19:47+00:00",
    "payload": {
      "reconnect_after": ""  # FIXME: timestamp, never again, etc
    }
}
```
