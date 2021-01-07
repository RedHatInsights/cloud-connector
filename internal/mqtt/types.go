package mqtt

type ConnectorMessage struct {
	MessageType string      `json:"type"`       // handshake, handshake_response
	MessageID   string      `json:"message_id"` // uuid
	ClientID    string      `json:"client_id"`  // uuid...i think
	Version     int         `json:"version"`
	Payload     interface{} `json:"payload"`
}

type HostHandshake struct {
	Type           string         `json:"type"`
	CanonicalFacts CanonicalFacts `json:"canonical_facts"`
}

type CanonicalFacts struct {
	InsightsID string `json:"insights_id"`
	MachineID  string `json:"machine_id"`
}
