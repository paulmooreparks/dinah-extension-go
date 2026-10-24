package extension

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Handler func(context.Context, *Host, json.RawMessage) (any, error)

func Serve(ctx context.Context, manifest Manifest, handlers map[string]Handler) error {
	if err := manifest.Validate(); err != nil {
		return err
	}
	server := mcp.NewServer(&mcp.Implementation{Name: manifest.ID, Version: manifest.Version}, nil)
	for _, action := range manifest.Actions {
		action := action
		handler := handlers[action.Name]
		if handler == nil {
			return fmt.Errorf("missing handler %s", action.Name)
		}
		server.AddTool(&mcp.Tool{Name: action.Name, Title: action.Title, Description: action.Description, InputSchema: action.InputSchema, OutputSchema: action.OutputSchema, Meta: mcp.Meta{"com.parkscomputing.dinah/action": action}}, func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			encoded, _ := json.Marshal(request.Params.Meta[ScopeMeta])
			var scope Scope
			if json.Unmarshal(encoded, &scope) != nil || scope.Token == "" {
				return nil, errors.New("a Dinah host scope is required")
			}
			out, err := handler(ctx, &Host{Scope: scope}, request.Params.Arguments)
			if err != nil {
				return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}}}, nil
			}
			data, err := json.Marshal(out)
			if err != nil {
				return nil, err
			}
			return &mcp.CallToolResult{StructuredContent: out, Content: []mcp.Content{&mcp.TextContent{Text: string(data)}}}, nil
		})
	}
	return server.Run(ctx, &mcp.StdioTransport{})
}
