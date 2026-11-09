package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type bearerRoundTripper struct {
	token string
	base  http.RoundTripper
}

func (b bearerRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return b.base.RoundTrip(r)
}

func readToken(token, tokenFile string) (string, error) {
	if tokenFile == "" {
		return strings.TrimSpace(token), nil
	}
	raw, err := os.ReadFile(tokenFile)
	if err != nil {
		return "", fmt.Errorf("read token file: %w", err)
	}
	return strings.TrimSpace(string(raw)), nil
}

func connectRemoteMCP(ctx context.Context, endpoint, token string) (*mcp.ClientSession, error) {
	httpClient := http.DefaultClient
	if token != "" {
		httpClient = &http.Client{Transport: bearerRoundTripper{token: token, base: http.DefaultTransport}}
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "flowpilot-bridge", Version: appVersion}, nil)
	cs, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: endpoint, HTTPClient: httpClient}, nil)
	if err != nil {
		return nil, fmt.Errorf("connect to running FlowPilot at %s: %w", endpoint, err)
	}
	return cs, nil
}

func newBridgeServer(ctx context.Context, remote *mcp.ClientSession) (*mcp.Server, error) {
	info := remote.InitializeResult()
	server := mcp.NewServer(&mcp.Implementation{Name: info.ServerInfo.Name, Title: info.ServerInfo.Title, Version: info.ServerInfo.Version},
		&mcp.ServerOptions{Instructions: info.Instructions})
	for tool, err := range remote.Tools(ctx, nil) {
		if err != nil {
			return nil, fmt.Errorf("list remote tools: %w", err)
		}
		name := tool.Name
		server.AddTool(tool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return remote.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: req.Params.Arguments})
		})
	}
	for res, err := range remote.Resources(ctx, nil) {
		if err != nil {
			return nil, fmt.Errorf("list remote resources: %w", err)
		}
		server.AddResource(res, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			return remote.ReadResource(ctx, &mcp.ReadResourceParams{URI: req.Params.URI})
		})
	}
	return server, nil
}

func runMCPBridge(ctx context.Context, endpoint, token string) int {
	remote, err := connectRemoteMCP(ctx, endpoint, token)
	if err != nil {
		fmt.Fprintf(os.Stderr, "flowpilot mcp: %v\n", err)
		return 1
	}
	defer remote.Close()
	server, err := newBridgeServer(ctx, remote)
	if err != nil {
		fmt.Fprintf(os.Stderr, "flowpilot mcp: %v\n", err)
		return 1
	}
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil && ctx.Err() == nil {
		fmt.Fprintf(os.Stderr, "flowpilot mcp bridge: %v\n", err)
		return 1
	}
	return 0
}
