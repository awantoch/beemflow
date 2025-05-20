package main

import (
	"context"
	"encoding/json"

	"github.com/awantoch/beemflow/api"
	"github.com/awantoch/beemflow/docs"
	mcpserver "github.com/awantoch/beemflow/mcp"
	"github.com/awantoch/beemflow/registry"
	"github.com/google/uuid"
	mcp "github.com/metoro-io/mcp-golang"
)

// Define a named struct for describe tool arguments
type DescribeArgs struct {
	Type string `json:"type"`
}

// buildMCPToolRegistrations returns all tool registrations for the MCP server.
func buildMCPToolRegistrations() []mcpserver.ToolRegistration {
	svc := api.NewFlowService()

	// Define all MCP tools and their metadata
	type toolDef struct {
		ID, Desc string
		Handler  any
	}
	defs := []toolDef{
		// SPEC tool: returns the full BeemFlow protocol SPEC
		{ID: "spec", Desc: "BeemFlow Protocol & Specification", Handler: func(ctx context.Context, args mcpserver.EmptyArgs) (*mcp.ToolResponse, error) {
			return mcp.NewToolResponse(mcp.NewTextContent(docs.BeemflowSpec)), nil
		}},
		{ID: registry.InterfaceIDListFlows, Desc: registry.InterfaceDescListFlows, Handler: func(ctx context.Context, args mcpserver.EmptyArgs) (*mcp.ToolResponse, error) {
			flows, err := svc.ListFlows(ctx)
			if err != nil {
				return nil, err
			}
			b, err := json.Marshal(map[string]any{"flows": flows})
			if err != nil {
				return nil, err
			}
			return mcp.NewToolResponse(mcp.NewTextContent(string(b))), nil
		}},
		{ID: registry.InterfaceIDGetFlow, Desc: registry.InterfaceDescGetFlow, Handler: func(ctx context.Context, args mcpserver.GetFlowArgs) (*mcp.ToolResponse, error) {
			flow, err := svc.GetFlow(ctx, args.Name)
			if err != nil {
				return nil, err
			}
			b, err := json.Marshal(flow)
			if err != nil {
				return nil, err
			}
			return mcp.NewToolResponse(mcp.NewTextContent(string(b))), nil
		}},
		{ID: registry.InterfaceIDValidateFlow, Desc: registry.InterfaceDescValidateFlow, Handler: func(ctx context.Context, args mcpserver.ValidateFlowArgs) (*mcp.ToolResponse, error) {
			err := svc.ValidateFlow(ctx, args.Name)
			if err != nil {
				return nil, err
			}
			return mcp.NewToolResponse(mcp.NewTextContent("valid")), nil
		}},
		{ID: registry.InterfaceIDGraphFlow, Desc: registry.InterfaceDescGraphFlow, Handler: func(ctx context.Context, args mcpserver.GraphFlowArgs) (*mcp.ToolResponse, error) {
			graph, err := svc.GraphFlow(ctx, args.Name)
			if err != nil {
				return nil, err
			}
			return mcp.NewToolResponse(mcp.NewTextContent(graph)), nil
		}},
		{ID: registry.InterfaceIDStartRun, Desc: registry.InterfaceDescStartRun, Handler: func(ctx context.Context, args mcpserver.StartRunArgs) (*mcp.ToolResponse, error) {
			id, err := svc.StartRun(ctx, args.FlowName, args.Event)
			if err != nil {
				return nil, err
			}
			b, err := json.Marshal(map[string]any{"runID": id.String()})
			if err != nil {
				return nil, err
			}
			return mcp.NewToolResponse(mcp.NewTextContent(string(b))), nil
		}},
		{ID: registry.InterfaceIDGetRun, Desc: registry.InterfaceDescGetRun, Handler: func(ctx context.Context, args mcpserver.GetRunArgs) (*mcp.ToolResponse, error) {
			id, _ := uuid.Parse(args.RunID)
			run, err := svc.GetRun(ctx, id)
			if err != nil {
				return nil, err
			}
			b, err := json.Marshal(run)
			if err != nil {
				return nil, err
			}
			return mcp.NewToolResponse(mcp.NewTextContent(string(b))), nil
		}},
		{ID: registry.InterfaceIDPublishEvent, Desc: registry.InterfaceDescPublishEvent, Handler: func(ctx context.Context, args mcpserver.PublishEventArgs) (*mcp.ToolResponse, error) {
			err := svc.PublishEvent(ctx, args.Topic, args.Payload)
			if err != nil {
				return nil, err
			}
			return mcp.NewToolResponse(mcp.NewTextContent("published")), nil
		}},
		{ID: registry.InterfaceIDResumeRun, Desc: registry.InterfaceDescResumeRun, Handler: func(ctx context.Context, args mcpserver.ResumeRunArgs) (*mcp.ToolResponse, error) {
			out, err := svc.ResumeRun(ctx, args.Token, args.Event)
			if err != nil {
				return nil, err
			}
			b, err := json.Marshal(out)
			if err != nil {
				return nil, err
			}
			return mcp.NewToolResponse(mcp.NewTextContent(string(b))), nil
		}},
		// Replace the describe handler to use DescribeArgs
		{ID: registry.InterfaceIDDescribe, Desc: registry.InterfaceDescMetadata, Handler: func(ctx context.Context, args DescribeArgs) (*mcp.ToolResponse, error) {
			all := registry.AllInterfaces()
			var filtered []registry.InterfaceMeta
			if args.Type == "" {
				filtered = all
			} else {
				for _, m := range all {
					if string(m.Type) == args.Type {
						filtered = append(filtered, m)
					}
				}
			}
			b, err := json.Marshal(filtered)
			if err != nil {
				return nil, err
			}
			return mcp.NewToolResponse(mcp.NewTextContent(string(b))), nil
		}},
	}
	// Build registrations and metadata in one pass
	regs := make([]mcpserver.ToolRegistration, 0, len(defs))
	for _, d := range defs {
		regs = append(regs, mcpserver.ToolRegistration{Name: d.ID, Description: d.Desc, Handler: d.Handler})
		registry.RegisterInterface(registry.InterfaceMeta{ID: d.ID, Type: registry.MCP, Use: d.ID, Description: d.Desc})
	}
	return regs
}
