package mqtt

type HandshakeMessage struct {
	MessageType string      `json:"type"`       // handshake
	MessageID   string      `json:"message_id"` // uuid
	Version     int         `json:"version"`
	Payload     interface{} `json:"payload"`
}

type HostHandshakePayload struct {
	CanonicalFacts CanonicalFacts `json:"canonical_facts"`
}

type ProxyHandshakePayload struct {
	CatalogServiceFacts CatalogServiceFacts `json:"catalog_service_facts"`
}

type CanonicalFacts struct {
	InsightsID string `json:"insights_id"`
	MachineID  string `json:"machine_id"`
}

type CatalogServiceFacts struct {
	SourtcesType    string `json:"sources_type"`
	ApplicationType string `json:"application_type"`
}

type ConnectorMessage struct {
	MessageType string      `json:"type"`       // handshake, handshake_response
	MessageID   string      `json:"message_id"` // uuid
	Version     int         `json:"version"`
	Directive   string      `json:"directive"`
	Payload     interface{} `json:"payload"`
}
