package mqtt

import (
	"time"
)

type ControlMessage struct {
	MessageType string      `json:"type"`
	MessageID   string      `json:"message_id"`
	Version     int         `json:"version"`
	Sent        time.Time   `json:"sent"`
	Content     interface{} `json:"content"`
}

type ConnectionStatusMessageContent struct {
	CanonicalFacts  CanonicalFacts `json:"canonical_facts"`
	Dispatchers     Dispatchers    `json:"dispatchers"`
	ConnectionState string         `json:"state"`
}

type Dispatchers map[string]map[string]string

type CommandMessageContent struct {
	Command   string      `json:"command"`
	Arguments interface{} `json:"arguments"`
}

type EventMessage struct {
	MessageType string    `json:"type"`
	MessageID   string    `json:"message_id"`
	ResponseTo  string    `json:"response_to"`
	Version     int       `json:"version"`
	Sent        time.Time `json:"sent"`
	Content     string    `json:"content"`
}

type EventMessageContent string // FIXME:  interface{} ??

type CanonicalFacts struct {
	InsightsID            string   `json:"insights_id"`
	MachineID             string   `json:"machine_id"`
	SubscriptionManagerID string   `json:"subscription_manager_id"`
	SatelliteID           string   `json:"satellite_id"`
	Fqdn                  string   `json:"fqdn"`
	BiosID                string   `json:"bios_uuid"`
	IpAddresses           []string `json:"ip_addresses"`
	MacAddresses          []string `json:"mac_addresses"`
}

type CatalogServiceFacts struct {
	SourcesType     string `json:"sources_type"`
	ApplicationType string `json:"application_type"`
}

type DataMessage struct {
	MessageType string      `json:"type"`
	MessageID   string      `json:"message_id"`
	ResponseTo  string      `json:"response_to"`
	Version     int         `json:"version"`
	Sent        time.Time   `json:"sent"`
	Directive   string      `json:"directive"`
	Metadata    interface{} `json:"metadata"`
	Content     interface{} `json:"content"`
}
