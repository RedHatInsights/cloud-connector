package mqtt

type ControlMessage struct {
	MessageType string      `json:"type"`
	MessageID   string      `json:"message_id"` // uuid
	Version     int         `json:"version"`
	Sent        string      `json:"sent"`
	Content     interface{} `json:"content"`
}

type ConnectionStatusMessageContent struct {
	CanonicalFacts  CanonicalFacts `json:"canonical_facts"`
	Dispatchers     Dispatchers    `json:"dispatchers"`
	ConnectionState string         `json:"state"`
}

type Dispatchers map[string]string

type CommandMessageContent struct {
	Command   string      `json:"command"`
	Arguments interface{} `json:"arguments"`
}

type EventMessageContent string // FIXME:  interface{} ??

type CanonicalFacts struct {
	InsightsID            string   `json:"insights_id"`
	MachineID             string   `json:"machine_id"`
	BiosID                string   `json:"bios_uuid"`
	SubscriptionManagerID string   `json:"subscription_manager_id"`
	IpAddresses           []string `json:"ip_addresses"`
	MacAddresses          []string `json:"mac_addresses"`
	Fqdn                  string   `json:"fqdn"`
}

type CatalogServiceFacts struct {
	SourcesType     string `json:"sources_type"`
	ApplicationType string `json:"application_type"`
}

type DataMessage struct {
	MessageType string      `json:"type"`
	MessageID   string      `json:"message_id"` // uuid
	Version     int         `json:"version"`
	Sent        string      `json:"sent"`
	Directive   string      `json:"directive"`
	Content     interface{} `json:"content"`
}
