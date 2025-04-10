package main

import (
	"context"
	"log"
	"os"

	"github.com/Khan/genqlient/generate"
	"github.com/suessflorian/gqlfetch"
)

var GRAPHQL_API_ENDPOINT string

func init() {
	GRAPHQL_API_ENDPOINT, _ = os.LookupEnv("GRAPHQL_API_ENDPOINT")
	if GRAPHQL_API_ENDPOINT == "" {
		GRAPHQL_API_ENDPOINT = "https://bff.resim.ai/graphql"
	}
}

func main() {
	log.Printf("Downloading GraphQL schema from %s", GRAPHQL_API_ENDPOINT)
	schema, err := gqlfetch.BuildClientSchema(context.Background(), GRAPHQL_API_ENDPOINT, false)
	if err != nil {
		log.Fatalf("Failed to fetch schema: %s", err)
	}

	// For some reason, gqlfetch does NOT include the root schema, which leads to genqlient
	// failing on 'Schema does not support operation type "query"'
	schema += `schema {
			query: RootQueryType
			mutation: RootMutationType
			subscription: RootSubscriptionType
		}`

	schemaFile := "../schema.graphql"
	if err := os.WriteFile(schemaFile, []byte(schema), 0644); err != nil {
		log.Fatalf("Failed to write schema file: %s", err)
	}

	generate.Main()

	log.Println("GraphQL client generated")

	if err := os.Remove(schemaFile); err != nil {
		log.Fatalf("failed to cleanup up schema file %s: %s", schemaFile, err)
	}
}

//go:generate go run github.com/resim-ai/api-client/bff/cmd
