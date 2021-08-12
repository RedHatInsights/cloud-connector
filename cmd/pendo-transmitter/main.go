package main

import (
	"fmt"
	"context"
	"encoding/json"

	cr "github.com/RedHatInsights/cloud-connector/internal/connection_repository"
	"github.com/RedHatInsights/cloud-connector/internal/domain"
)

var S []requestBody

type requestBody struct {
	AccountID	domain.AccountID	`json:"accountId"`
	Values		requestValues	`json:"values"`
}

type requestValues struct {
	ConnCount  int	`json:"connectionCount"`
}


func connectionCountProcessor(ctx context.Context, account domain.AccountID, count int) error {
	fmt.Printf("%s - %d\n", account, count)
	S = append(S, requestBody{AccountID: account, Values: requestValues{ConnCount: count}})
	return nil
}


func main() {
	fmt.Println("Build successful!")

	cr.StartConnectedAccountReport("545412,4666546", connectionCountProcessor)
	m,_ := json.Marshal(S)
	fmt.Println(string(m))
}

