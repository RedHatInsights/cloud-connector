package main

type ConnectionEvent struct {
	Event    string `json:event`
	ClientId string `json:client_id`
}

type Credentials struct {
	Username string `json:username`
	Password string `json:password`
}
