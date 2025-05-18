package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/awantoch/beemflow/api"
	"github.com/awantoch/beemflow/engine"
	"github.com/google/uuid"
	mcp "github.com/metoro-io/mcp-golang"
)

// Argument structs for MCP handlers

type EmptyArgs struct{}
type GetFlowArgs struct {
	Name string `json:"name"`
}
type ValidateFlowArgs struct {
	Name string `json:"name"`
}
type GraphFlowArgs struct {
	Name string `json:"name"`
}
type StartRunArgs struct {
	FlowName string         `json:"flowName"`
	Event    map[string]any `json:"event"`
}
type GetRunArgs struct {
	RunID string `json:"runID"`
}
type PublishEventArgs struct {
	Topic   string         `json:"topic"`
	Payload map[string]any `json:"payload"`
}

// registerAllMCPTools registers all BeemFlow operations as MCP tools.
func registerAllMCPTools(server *mcp.Server, eng *engine.Engine) {
	_ = server.RegisterTool("listFlows", "List all flows", MCPHandler_ListFlows(eng))
	_ = server.RegisterTool("getFlow", "Get a flow by name", MCPHandler_GetFlow(eng))
	_ = server.RegisterTool("validateFlow", "Validate a flow by name", MCPHandler_ValidateFlow(eng))
	_ = server.RegisterTool("graphFlow", "Get DOT graph for a flow", MCPHandler_GraphFlow(eng))
	_ = server.RegisterTool("startRun", "Start a new run for a flow", MCPHandler_StartRun(eng))
	_ = server.RegisterTool("getRun", "Get a run by ID", MCPHandler_GetRun(eng))
	_ = server.RegisterTool("publishEvent", "Publish an event to a topic", MCPHandler_PublishEvent(eng))
}

func MCPHandler_ListFlows(eng *engine.Engine) func(ctx context.Context, args EmptyArgs) (*mcp.ToolResponse, error) {
	return func(ctx context.Context, args EmptyArgs) (*mcp.ToolResponse, error) {
		flows, err := api.ListFlows(ctx)
		if err != nil {
			return nil, err
		}
		jsonBytes, err := json.Marshal(map[string]any{"flows": flows})
		if err != nil {
			return nil, err
		}
		return mcp.NewToolResponse(mcp.NewTextContent(string(jsonBytes))), nil
	}
}
func MCPHandler_GetFlow(eng *engine.Engine) func(ctx context.Context, args GetFlowArgs) (*mcp.ToolResponse, error) {
	return func(ctx context.Context, args GetFlowArgs) (*mcp.ToolResponse, error) {
		path := "flows/" + args.Name + ".flow.yaml"
		b, err := os.ReadFile(path)
		fmt.Printf("[DEBUG] MCPHandler_GetFlow reading: %s\n", path)
		if err != nil {
			fmt.Printf("[DEBUG] MCPHandler_GetFlow read error: %v\n", err)
		} else {
			fmt.Printf("[DEBUG] MCPHandler_GetFlow file contents:\n%s\n", string(b))
		}
		flow, err := api.GetFlow(ctx, args.Name)
		if err != nil {
			return nil, err
		}
		jsonBytes, err := json.Marshal(flow)
		if err != nil {
			return nil, err
		}
		return mcp.NewToolResponse(mcp.NewTextContent(string(jsonBytes))), nil
	}
}
func MCPHandler_ValidateFlow(eng *engine.Engine) func(ctx context.Context, args ValidateFlowArgs) (*mcp.ToolResponse, error) {
	return func(ctx context.Context, args ValidateFlowArgs) (*mcp.ToolResponse, error) {
		err := api.ValidateFlow(ctx, args.Name)
		if err != nil {
			return nil, err
		}
		return mcp.NewToolResponse(mcp.NewTextContent("valid")), nil
	}
}
func MCPHandler_GraphFlow(eng *engine.Engine) func(ctx context.Context, args GraphFlowArgs) (*mcp.ToolResponse, error) {
	return func(ctx context.Context, args GraphFlowArgs) (*mcp.ToolResponse, error) {
		return mcp.NewToolResponse(mcp.NewTextContent("stub: graphFlow")), nil
	}
}
func MCPHandler_StartRun(eng *engine.Engine) func(ctx context.Context, args StartRunArgs) (*mcp.ToolResponse, error) {
	return func(ctx context.Context, args StartRunArgs) (*mcp.ToolResponse, error) {
		runID, err := api.StartRun(ctx, args.FlowName, args.Event)
		if err != nil {
			return nil, err
		}
		jsonBytes, err := json.Marshal(map[string]any{"runID": runID.String()})
		if err != nil {
			return nil, err
		}
		return mcp.NewToolResponse(mcp.NewTextContent(string(jsonBytes))), nil
	}
}
func MCPHandler_GetRun(eng *engine.Engine) func(ctx context.Context, args GetRunArgs) (*mcp.ToolResponse, error) {
	return func(ctx context.Context, args GetRunArgs) (*mcp.ToolResponse, error) {
		runID, _ := uuid.Parse(args.RunID)
		run, err := api.GetRun(ctx, runID)
		if err != nil {
			return nil, err
		}
		jsonBytes, err := json.Marshal(run)
		if err != nil {
			return nil, err
		}
		return mcp.NewToolResponse(mcp.NewTextContent(string(jsonBytes))), nil
	}
}
func MCPHandler_PublishEvent(eng *engine.Engine) func(ctx context.Context, args PublishEventArgs) (*mcp.ToolResponse, error) {
	return func(ctx context.Context, args PublishEventArgs) (*mcp.ToolResponse, error) {
		err := api.PublishEvent(ctx, args.Topic, args.Payload)
		if err != nil {
			return nil, err
		}
		return mcp.NewToolResponse(mcp.NewTextContent("published")), nil
	}
}
