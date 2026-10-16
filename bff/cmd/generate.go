package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/Khan/genqlient/generate"
	"github.com/suessflorian/gqlfetch"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

var GRAPHQL_API_ENDPOINT string

func init() {
	GRAPHQL_API_ENDPOINT, _ = os.LookupEnv("GRAPHQL_API_ENDPOINT")
	if GRAPHQL_API_ENDPOINT == "" {
		GRAPHQL_API_ENDPOINT = "https://bff.resim.ai/graphql"
	}
}

func main() {
	schema, err := loadSchema(context.Background(), os.Getenv("GRAPHQL_SCHEMA_FILE"), GRAPHQL_API_ENDPOINT)
	if err != nil {
		log.Fatal(err)
	}

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

func loadSchema(ctx context.Context, path, endpoint string) (string, error) {
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read GraphQL schema: %w", err)
		}
		if _, err := gqlparser.LoadSchema(&ast.Source{Name: path, Input: string(data)}); err != nil {
			return "", fmt.Errorf("invalid GraphQL schema: %w", err)
		}
		return string(data), nil
	}

	schema, err := gqlfetch.BuildClientSchema(ctx, endpoint, false)
	if err != nil {
		return "", fmt.Errorf("fetch GraphQL schema: %w", err)
	}
	// gqlfetch omits the root schema declaration.
	schema += `schema {
		query: RootQueryType
		mutation: RootMutationType
		subscription: RootSubscriptionType
	}`
	return schema, nil
}

//go:generate go run github.com/resim-ai/api-client/bff/cmd
