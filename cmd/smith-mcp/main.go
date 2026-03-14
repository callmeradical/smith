package main

import (
	"log"
	"os"

	client "smith/pkg/client/v1"
	"smith/pkg/mcp"
)

func main() {
	baseURL := os.Getenv("SMITH_API_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	token := os.Getenv("SMITH_API_TOKEN")

	c := client.NewClient(baseURL, token)
	s := mcp.NewSmithMCPServer(c)

	log.Printf("Smith MCP server starting on stdio")
	if err := s.Serve(); err != nil {
		log.Fatalf("Error serving MCP: %v", err)
	}
}
