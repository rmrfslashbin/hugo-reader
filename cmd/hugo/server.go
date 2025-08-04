package hugo

import (
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	mcp_golang "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
	"github.com/rmrfslashbin/mcp/hugo-reader/internal/cache"
	"github.com/rmrfslashbin/mcp/hugo-reader/internal/logging"
	cachetools "github.com/rmrfslashbin/mcp/hugo-reader/internal/tools/cache"
	"github.com/rmrfslashbin/mcp/hugo-reader/internal/tools/content"
	"github.com/rmrfslashbin/mcp/hugo-reader/internal/tools/discovery"
	"github.com/rmrfslashbin/mcp/hugo-reader/internal/tools/info"
	"github.com/rmrfslashbin/mcp/hugo-reader/internal/tools/search"
	"github.com/rmrfslashbin/mcp/hugo-reader/internal/tools/taxonomies"
	"github.com/rmrfslashbin/mcp/hugo-reader/internal/tools/terms"
	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the MCP server",
	Long: `Start the MCP server that provides access to Hugo sites.
The server runs until it receives a signal to shut down.`,
	RunE: runServer,
}

func init() {
	rootCmd.AddCommand(serverCmd)
}

func runServer(cmd *cobra.Command, args []string) error {
	// Create a logger
	logger := logging.New()

	// Create a channel to listen for OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)

	// Create error channel to capture server errors
	errChan := make(chan error, 1)

	// Create a new MCP server
	transport := stdio.NewStdioServerTransport()
	server := mcp_golang.NewServer(transport)

	// Create shared cache instance
	cacheInstance := cache.New(cache.WithLogger(logger))

	// Register all tools
	if err := registerTools(server, logger, cacheInstance); err != nil {
		logger.Error("Failed to register tools", "error", err)
		return err
	}

	logger.Info("Server starting with all tools registered")

	// Start server in a goroutine
	go func() {
		logger.Info("starting server...")
		if err := server.Serve(); err != nil {
			logger.Error("failed to serve", slog.String("error", err.Error()))
			errChan <- err
		}
	}()

	// Wait for either a signal or server error
	select {
	case sig := <-sigChan:
		logger.Info("Received signal", "signal", sig)
		// Gracefully shut down the server by closing the transport
		if err := transport.Close(); err != nil {
			logger.Error("Error closing transport", slog.String("error", err.Error()))
		}
	case err := <-errChan:
		if err != nil {
			logger.Error("Server error", "error", slog.String("error", err.Error()))
			return err
		}
	}

	logger.Info("Server shutdown complete")
	return nil
}

// registerTools registers all available tools with the MCP server
func registerTools(server *mcp_golang.Server, logger *slog.Logger, cacheInstance *cache.Cache) error {
	// Create tool instances
	taxonomiesTool, err := taxonomies.New(
		taxonomies.WithLogger(logger),
		taxonomies.WithCache(cacheInstance),
	)
	if err != nil {
		return fmt.Errorf("failed to create taxonomies tool: %w", err)
	}

	termsTool, err := terms.New(
		terms.WithLogger(logger),
		terms.WithCache(cacheInstance),
	)
	if err != nil {
		return fmt.Errorf("failed to create terms tool: %w", err)
	}

	contentTool, err := content.New(
		content.WithLogger(logger),
		content.WithCache(cacheInstance),
	)
	if err != nil {
		return fmt.Errorf("failed to create content tool: %w", err)
	}

	searchTool, err := search.New(
		search.WithLogger(logger),
		search.WithCache(cacheInstance),
	)
	if err != nil {
		return fmt.Errorf("failed to create search tool: %w", err)
	}

	cacheTool, err := cachetools.New(
		cacheInstance,
		cachetools.WithLogger(logger),
	)
	if err != nil {
		return fmt.Errorf("failed to create cache tool: %w", err)
	}

	discoveryTool, err := discovery.New(
		discovery.WithLogger(logger),
		discovery.WithCache(cacheInstance),
	)
	if err != nil {
		return fmt.Errorf("failed to create discovery tool: %w", err)
	}

	infoTool, err := info.New(
		GitCommit,
		info.WithLogger(logger),
		info.WithVersion("1.0.0"),
	)
	if err != nil {
		return fmt.Errorf("failed to create info tool: %w", err)
	}

	// Register tools with handler functions
	if err := server.RegisterTool(
		taxonomiesTool.Name(),
		taxonomiesTool.Description(),
		func(args *taxonomies.TaxonomiesRequest) (*mcp_golang.ToolResponse, error) {
			return taxonomiesTool.Execute(args)
		},
	); err != nil {
		return fmt.Errorf("failed to register taxonomies tool: %w", err)
	}

	if err := server.RegisterTool(
		termsTool.Name(),
		termsTool.Description(),
		func(args *terms.TaxonomyTermsRequest) (*mcp_golang.ToolResponse, error) {
			return termsTool.Execute(args)
		},
	); err != nil {
		return fmt.Errorf("failed to register terms tool: %w", err)
	}

	if err := server.RegisterTool(
		contentTool.Name(),
		contentTool.Description(),
		func(args *content.ContentRequest) (*mcp_golang.ToolResponse, error) {
			return contentTool.Execute(args)
		},
	); err != nil {
		return fmt.Errorf("failed to register content tool: %w", err)
	}

	if err := server.RegisterTool(
		searchTool.Name(),
		searchTool.Description(),
		func(args *search.SearchRequest) (*mcp_golang.ToolResponse, error) {
			return searchTool.Execute(args)
		},
	); err != nil {
		return fmt.Errorf("failed to register search tool: %w", err)
	}

	if err := server.RegisterTool(
		cacheTool.Name(),
		cacheTool.Description(),
		func(args *cachetools.ClearCacheRequest) (*mcp_golang.ToolResponse, error) {
			return cacheTool.Execute(args)
		},
	); err != nil {
		return fmt.Errorf("failed to register cache tool: %w", err)
	}

	if err := server.RegisterTool(
		discoveryTool.Name(),
		discoveryTool.Description(),
		func(args *discovery.DiscoveryRequest) (*mcp_golang.ToolResponse, error) {
			return discoveryTool.Execute(args)
		},
	); err != nil {
		return fmt.Errorf("failed to register discovery tool: %w", err)
	}

	if err := server.RegisterTool(
		infoTool.Name(),
		infoTool.Description(),
		func(args *info.InfoRequest) (*mcp_golang.ToolResponse, error) {
			return infoTool.Execute(args)
		},
	); err != nil {
		return fmt.Errorf("failed to register info tool: %w", err)
	}

	logger.Info("Successfully registered all tools", 
		"tools", []string{
			taxonomiesTool.Name(),
			termsTool.Name(), 
			contentTool.Name(),
			searchTool.Name(),
			cacheTool.Name(),
			discoveryTool.Name(),
			infoTool.Name(),
		})

	return nil
}

