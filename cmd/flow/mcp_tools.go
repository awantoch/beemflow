package main

import (
	"context"
	"encoding/json"

	"github.com/awantoch/beemflow/api"
	mcpserver "github.com/awantoch/beemflow/mcp"
	"github.com/google/uuid"
	mcp "github.com/metoro-io/mcp-golang"
)

// buildMCPToolRegistrations returns all tool registrations for the MCP server.
func buildMCPToolRegistrations() []mcpserver.ToolRegistration {
	svc := api.NewFlowService()
	return []mcpserver.ToolRegistration{
		{
			Name:        "listFlows",
			Description: "List all flows",
			Handler: func(ctx context.Context, args mcpserver.EmptyArgs) (*mcp.ToolResponse, error) {
				flows, err := svc.ListFlows(ctx)
				if err != nil {
					return nil, err
				}
				jsonBytes, err := json.Marshal(map[string]any{"flows": flows})
				if err != nil {
					return nil, err
				}
				return mcp.NewToolResponse(mcp.NewTextContent(string(jsonBytes))), nil
			},
		},
		{
			Name:        "getFlow",
			Description: "Get a flow by name",
			Handler: func(ctx context.Context, args mcpserver.GetFlowArgs) (*mcp.ToolResponse, error) {
				flow, err := svc.GetFlow(ctx, args.Name)
				if err != nil {
					return nil, err
				}
				jsonBytes, err := json.Marshal(flow)
				if err != nil {
					return nil, err
				}
				return mcp.NewToolResponse(mcp.NewTextContent(string(jsonBytes))), nil
			},
		},
		{
			Name:        "validateFlow",
			Description: "Validate a flow by name",
			Handler: func(ctx context.Context, args mcpserver.ValidateFlowArgs) (*mcp.ToolResponse, error) {
				err := svc.ValidateFlow(ctx, args.Name)
				if err != nil {
					return nil, err
				}
				return mcp.NewToolResponse(mcp.NewTextContent("valid")), nil
			},
		},
		{
			Name:        "graphFlow",
			Description: "Get the Mermaid diagram for a flow",
			Handler: func(ctx context.Context, args mcpserver.GraphFlowArgs) (*mcp.ToolResponse, error) {
				graph, err := svc.GraphFlow(ctx, args.Name)
				if err != nil {
					return nil, err
				}
				return mcp.NewToolResponse(mcp.NewTextContent(graph)), nil
			},
		},
		{
			Name:        "startRun",
			Description: "Start a new run for a flow",
			Handler: func(ctx context.Context, args mcpserver.StartRunArgs) (*mcp.ToolResponse, error) {
				runID, err := svc.StartRun(ctx, args.FlowName, args.Event)
				if err != nil {
					return nil, err
				}
				jsonBytes, err := json.Marshal(map[string]any{"runID": runID.String()})
				if err != nil {
					return nil, err
				}
				return mcp.NewToolResponse(mcp.NewTextContent(string(jsonBytes))), nil
			},
		},
		{
			Name:        "getRun",
			Description: "Get a run by ID",
			Handler: func(ctx context.Context, args mcpserver.GetRunArgs) (*mcp.ToolResponse, error) {
				runID, _ := uuid.Parse(args.RunID)
				run, err := svc.GetRun(ctx, runID)
				if err != nil {
					return nil, err
				}
				jsonBytes, err := json.Marshal(run)
				if err != nil {
					return nil, err
				}
				return mcp.NewToolResponse(mcp.NewTextContent(string(jsonBytes))), nil
			},
		},
		{
			Name:        "publishEvent",
			Description: "Publish an event to a topic",
			Handler: func(ctx context.Context, args mcpserver.PublishEventArgs) (*mcp.ToolResponse, error) {
				err := svc.PublishEvent(ctx, args.Topic, args.Payload)
				if err != nil {
					return nil, err
				}
				return mcp.NewToolResponse(mcp.NewTextContent("published")), nil
			},
		},
	}
}
