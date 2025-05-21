package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/awantoch/beemflow/api"
	"github.com/awantoch/beemflow/config"
	"github.com/awantoch/beemflow/docs"
	"github.com/awantoch/beemflow/logger"
	mcpserver "github.com/awantoch/beemflow/mcp"
	"github.com/awantoch/beemflow/registry"
	"github.com/google/uuid"
	mcp "github.com/metoro-io/mcp-golang"
	"github.com/spf13/cobra"
)

// newMCPCmd creates the 'mcp' subcommand and its subcommands.
func newMCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "MCP server commands",
	}

	var configFile = &configPath

	cmd.AddCommand(
		newMCPServeCmd(),
		&cobra.Command{
			Use:   "search [query]",
			Short: "Search for MCP servers in the Smithery registry",
			Args:  cobra.MaximumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				query := ""
				if len(args) > 0 {
					query = args[0]
				}
				ctx := context.Background()
				apiKey := os.Getenv("SMITHERY_API_KEY")
				if apiKey == "" {
					return fmt.Errorf("environment variable SMITHERY_API_KEY must be set")
				}
				client := registry.NewSmitheryRegistry(apiKey, "")
				entries, err := client.ListServers(ctx, registry.ListOptions{Query: query, PageSize: 50})
				if err != nil {
					return err
				}
				logger.User("NAME\tDESCRIPTION\tENDPOINT")
				for _, s := range entries {
					logger.User("%s\t%s\t%s", s.Name, s.Description, s.Endpoint)
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "install <serverName>",
			Short: "Install an MCP server from the Smithery registry",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				qn := args[0]
				// Read existing config as raw JSON (preserve only user overrides)
				var doc map[string]any
				data, err := os.ReadFile(*configFile)
				if err != nil {
					if os.IsNotExist(err) {
						doc = map[string]any{}
					} else {
						return err
					}
				} else {
					if err := json.Unmarshal(data, &doc); err != nil {
						return fmt.Errorf("failed to parse %s: %w", *configFile, err)
					}
				}
				// Ensure mcpServers map exists
				mcpMap, ok := doc["mcpServers"].(map[string]any)
				if !ok {
					mcpMap = map[string]any{}
				}
				// Fetch spec from Smithery
				ctx := context.Background()
				apiKey := os.Getenv("SMITHERY_API_KEY")
				if apiKey == "" {
					return fmt.Errorf("environment variable SMITHERY_API_KEY must be set")
				}
				client := registry.NewSmitheryRegistry(apiKey, "")
				spec, err := client.GetServerSpec(ctx, qn)
				if err != nil {
					return err
				}
				// Patch mcpServers
				mcpMap[qn] = spec
				doc["mcpServers"] = mcpMap
				// Write updated config
				out, err := json.MarshalIndent(doc, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to serialize config: %w", err)
				}
				if err := os.WriteFile(*configFile, out, 0644); err != nil {
					return fmt.Errorf("failed to write %s: %w", *configFile, err)
				}
				logger.User("Installed MCP server %s to %s (mcpServers)", qn, *configFile)
				return nil
			},
		},
		&cobra.Command{
			Use:   "list",
			Short: "List all MCP servers",
			RunE: func(cmd *cobra.Command, args []string) error {
				// Load config to get installed MCP servers
				cfg, err := config.LoadConfig(*configFile)
				if err != nil && !os.IsNotExist(err) {
					return err
				}
				ctx := context.Background()
				logger.User("REGISTRY\tNAME\tDESCRIPTION\tKIND\tENDPOINT")
				if cfg != nil && cfg.MCPServers != nil {
					for name, spec := range cfg.MCPServers {
						logger.User("config\t%s\t%s\t%s\t%s", name, "", spec.Transport, spec.Endpoint)
					}
				}
				localMgr := registry.NewLocalRegistry("")
				servers, err := localMgr.ListMCPServers(ctx, registry.ListOptions{PageSize: 100})
				if err == nil {
					for _, s := range servers {
						logger.User("%s\t%s\t%s\t%s\t%s", s.Registry, s.Name, s.Description, s.Kind, s.Endpoint)
					}
				}
				return nil
			},
		},
	)
	return cmd
}

// ---- MCP Serve Command (from mcp_serve.go) ----

func newMCPServeCmd() *cobra.Command {
	var stdio bool
	var addr string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve BeemFlow as an MCP server (HTTP or stdio)",
		Run: func(cmd *cobra.Command, args []string) {
			tools := buildMCPToolRegistrations()
			if err := mcpserver.Serve(configPath, debug, stdio, addr, tools); err != nil {
				log.Fatalf("MCP server failed: %v", err)
			}
		},
	}
	cmd.Flags().BoolVar(&stdio, "stdio", true, "serve over stdin/stdout instead of HTTP (default)")
	cmd.Flags().StringVar(&addr, "addr", ":9090", "listen address for HTTP mode")
	return cmd
}

// ---- MCP Tool Registrations (from mcp_tools.go) ----

type DescribeArgs struct {
	Type string `json:"type"`
}

func buildMCPToolRegistrations() []mcpserver.ToolRegistration {
	svc := api.NewFlowService()

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
			// If default empty flow, inject on:null into JSON
			if flow.Name == "" && len(flow.Steps) == 0 {
				var m map[string]interface{}
				if err := json.Unmarshal(b, &m); err == nil {
					if _, ok := m["on"]; !ok {
						m["on"] = nil
					}
					if b2, err2 := json.Marshal(m); err2 == nil {
						b = b2
					}
				}
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
	regs := make([]mcpserver.ToolRegistration, 0, len(defs))
	for _, d := range defs {
		regs = append(regs, mcpserver.ToolRegistration{Name: d.ID, Description: d.Desc, Handler: d.Handler})
		registry.RegisterInterface(registry.InterfaceMeta{ID: d.ID, Type: registry.MCP, Use: d.ID, Description: d.Desc})
	}
	return regs
}
