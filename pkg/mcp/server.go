package mcp

import (
	"context"
	"fmt"

	mcp "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
	api "smith/pkg/api/v1"
	client "smith/pkg/client/v1"
)

type SmithMCPServer struct {
	client client.Interface
	server *mcp.Server
}

type GetLoopArgs struct {
	LoopID string `json:"loop_id" jsonschema:"required,description=The ID of the loop to get"`
}

type CreateLoopArgs struct {
	Title       string `json:"title" jsonschema:"required,description=The title of the loop"`
	Description string `json:"description" jsonschema:"description=The description of the loop"`
	SourceType  string `json:"source_type" jsonschema:"required,description=The source type (e.g. github_issue)"`
	SourceRef   string `json:"source_ref" jsonschema:"required,description=The source reference (e.g. repo#123)"`
}

func NewSmithMCPServer(c client.Interface) *SmithMCPServer {
	transport := stdio.NewStdioServerTransport()
	s := mcp.NewServer(transport)

	smithServer := &SmithMCPServer{
		client: c,
		server: s,
	}

	smithServer.registerTools()

	return smithServer
}

func (s *SmithMCPServer) registerTools() {
	// List Loops
	_ = s.server.RegisterTool("list_loops", "List all Smith loops", func(args struct{}) (*mcp.ToolResponse, error) {
		loops, err := s.client.ListLoops(context.Background())
		if err != nil {
			return nil, err
		}
		return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("%+v", loops))), nil
	})

	// Get Loop
	_ = s.server.RegisterTool("get_loop", "Get detailed information about a Smith loop", func(args GetLoopArgs) (*mcp.ToolResponse, error) {
		loop, err := s.client.GetLoop(context.Background(), args.LoopID)
		if err != nil {
			return nil, err
		}
		return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("%+v", loop))), nil
	})

	// Create Loop
	_ = s.server.RegisterTool("create_loop", "Create a new Smith loop", func(args CreateLoopArgs) (*mcp.ToolResponse, error) {
		req := api.LoopCreateRequest{
			Title:       args.Title,
			Description: args.Description,
			SourceType:  args.SourceType,
			SourceRef:   args.SourceRef,
		}

		res, err := s.client.CreateLoop(context.Background(), req)
		if err != nil {
			return nil, err
		}
		return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("%+v", res))), nil
	})
}

func (s *SmithMCPServer) Serve() error {
	return s.server.Serve()
}
