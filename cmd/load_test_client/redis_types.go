package main


type ConnectionEvent struct {
    Event string `json:event`
    ClientId string `json:client_id`
}
