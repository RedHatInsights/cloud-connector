package protocol

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
	CanonicalFacts  CanonicalFacts `json:"canonical_facts,omitempty"`
	Dispatchers     Dispatchers    `json:"dispatchers"`
	ConnectionState string         `json:"state"`
	Tags            Tags           `json:"tags"`
	ClientName      string         `json:"client_name"`
	ClientVersion   string         `json:"client_version"`
}

type Dispatchers map[string]map[string]string
type Tags map[string]string

type CommandMessageContent struct {
	Command   string      `json:"command"`
	Arguments interface{} `json:"arguments"`
	Message   string      `json:"message"`
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
	InsightsID            string   `json:"insights_id,omitempty"`
	MachineID             string   `json:"machine_id,omitempty"`
	SubscriptionManagerID string   `json:"subscription_manager_id,omitempty"`
	SatelliteID           string   `json:"satellite_id,omitempty"`
	Fqdn                  string   `json:"fqdn,omitempty"`
	BiosID                string   `json:"bios_uuid,omitempty"`
	IpAddresses           []string `json:"ip_addresses,omitempty"`
	MacAddresses          []string `json:"mac_addresses,omitempty"`
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
