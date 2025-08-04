# Hugo Reader

A Model Control Protocol (MCP) server that provides comprehensive access to Hugo static sites through their JSON endpoints.

## Features

- **7 Complete Tools** for Hugo site introspection
- **Smart Caching** with HTTP validation (ETag/Last-Modified) and 5-minute TTL
- **Hugo-Specific Intelligence** with multi-endpoint discovery and validation
- **Advanced Search** with Hugo-native indices and intelligent fallback to content scanning
- **Bulk Content Retrieval** with flexible response options (metadata/body/both)
- **Comprehensive Error Handling** with structured error objects and user-friendly messages
- **Cache Management** with statistics and manual control
- **Production-Ready** with extensive test coverage and MCP protocol compliance

## Requirements

- Go 1.24 or higher
- [MCP protocol](https://github.com/metoro-io/mcp-golang) support

## Installation

```bash
git clone https://github.com/rmrfslashbin/mcp/hugo-reader
cd hugo-reader
make build
```

## Configuration

The application uses environment variables for configuration. You can create a `.env` file in the project root:

```
LOG_LEVEL=debug  # Options: debug, info, warn, error (default: info)
MCP_SERVER_NAME=hugo-reader  # Custom server name (default: hugo-reader)
HUGO_READER_HTTP_TIMEOUT=10  # HTTP timeout in seconds (default: 10)
HUGO_READER_USER_AGENT=HugoReader/1.0.0  # User agent for HTTP requests
```

## Usage

Run the server:

```bash
./bin/hugo-reader server
```

The server communicates via stdin/stdout using the MCP protocol.

## Claude Desktop Configuration

To use this MCP server with Claude Desktop, add the following configuration to your `claude_desktop_config.json` file:

```json
{
  "mcpServers": {
    "hugo-reader": {
      "command": "/path/to/your/hugo-reader",
      "args": ["server"],
      "env": {
        "LOG_LEVEL": "info"
      }
    }
  }
}
```

### Example Configuration Locations

- **macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
- **Windows**: `%APPDATA%\Claude\claude_desktop_config.json`
- **Linux**: `~/.config/Claude/claude_desktop_config.json`

### Configuration Options

You can customize the MCP server behavior through environment variables:

```json
{
  "mcpServers": {
    "hugo-reader": {
      "command": "/path/to/your/hugo-reader",
      "args": ["server"],
      "env": {
        "LOG_LEVEL": "debug",
        "HUGO_READER_HTTP_TIMEOUT": "30",
        "HUGO_READER_USER_AGENT": "Claude-HugoReader/1.0.0"
      }
    }
  }
}
```

## Tools

### hugo_reader_get_taxonomies

Get all taxonomies defined in the Hugo site.

**Parameters:**
- `hugo_site_path`: Complete URL of the Hugo site (e.g., https://example.com)

**Example response:**
```json
{
  "success": true,
  "taxonomies": {
    "categories": "categories",
    "tags": "tags"
  }
}
```

### hugo_reader_get_taxonomy_terms

Get all terms for a specific taxonomy from a Hugo site.

**Parameters:**
- `hugo_site_path`: Complete URL of the Hugo site (e.g., https://example.com)
- `taxonomy`: The taxonomy name to retrieve terms for (e.g., "categories", "tags")

**Example response:**
```json
{
  "success": true,
  "taxonomy": "tags",
  "terms": [
    {
      "name": "technology",
      "path": "/tags/technology"
    },
    {
      "name": "personal",
      "path": "/tags/personal"
    }
  ]
}
```

### hugo_reader_get_content

Get content from a Hugo site by path.

**Parameters:**
- `hugo_site_path`: Complete URL of the Hugo site (e.g., https://example.com)
- `content_path`: Path to the content relative to the site root (e.g., "posts/my-post")

**Example response:**
```json
{
  "success": true,
  "content": {
    "path": "posts/my-post",
    "front_matter": {
      "title": "My Post",
      "date": "2023-01-01",
      "tags": ["technology", "personal"]
    },
    "content": "This is the content of the post..."
  }
}
```

### hugo_reader_search

Search content in a Hugo site by keyword with optional filters.

**Parameters:**
- `hugo_site_path`: Complete URL of the Hugo site (e.g., https://example.com)
- `query`: Search query string (will match case-insensitively against titles and content)
- `content_type` (optional): Content type to filter by (e.g., "posts", "pages")
- `taxonomy` (optional): Taxonomy name to filter by (e.g., "categories", "tags")
- `term` (optional): Taxonomy term to filter by (e.g., "technology", "personal")
- `limit` (optional): Maximum number of results to return (default: 10)

**Example response:**
```json
{
  "success": true,
  "results": [
    {
      "title": "My Technology Post",
      "path": "/posts/my-technology-post",
      "type": "posts",
      "date": "2023-01-01",
      "taxonomies": {
        "categories": ["technology"],
        "tags": ["tech", "web"]
      },
      "summary": "A brief summary of the post..."
    }
  ]
}
```

### hugo_reader_discover_site

Discover available content and structure in Hugo sites when you don't know what's available.

**Parameters:**
- `hugo_site_path`: Complete URL of the Hugo site (e.g., https://example.com)
- `discovery_type` (optional): Type of discovery - "overview", "sections", "pages", or "sitemap" (default: "overview")
- `limit` (optional): Maximum number of results to return (default: 50, max: 200)

**Example response:**
```json
{
  "success": true,
  "discovery_type": "pages",
  "results": [
    {
      "title": "My Post",
      "url": "/posts/my-post/",
      "path": "/posts/my-post/",
      "date": "2023-01-01",
      "section": "posts"
    }
  ],
  "metadata": {
    "discovery_method": "pages",
    "total_found": 25,
    "source": "index.json",
    "limited": false
  }
}
```

### hugo_reader_cache_manager

Manage cache for better performance and fresh data.

**Parameters:**
- `action`: Cache action - "clear", "stats", or "clean"
- `target` (optional): Specific site URL to target for clearing

**Example response:**
```json
{
  "success": true,
  "action": "stats",
  "cache_info": {
    "total_entries": 15,
    "total_size_bytes": 45678,
    "oldest_entry": "2023-01-01T12:00:00Z",
    "newest_entry": "2023-01-01T12:05:00Z"
  }
}
```

### hugo_reader_info

Get version, build, and runtime information about the Hugo Reader MCP server.

**Parameters:**
- `include_runtime` (optional): Include Go runtime information (default: false)
- `include_tools` (optional): Include list of available tools (default: false)

**Example response:**
```json
{
  "success": true,
  "info": {
    "name": "Hugo Reader MCP Server",
    "version": "1.0.0",
    "git_commit": "ab4fcf0",
    "build_time": "2023-01-01T12:00:00Z",
    "description": "Model Control Protocol server for Hugo static sites",
    "repository": "https://github.com/rmrfslashbin/mcp/hugo-reader",
    "mcp": {
      "protocol_version": "1.0",
      "transport": "stdio",
      "capabilities": ["tool_execution", "caching", "error_handling", "request_validation"]
    }
  },
  "timestamp": "2023-01-01T12:00:00Z"
}
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Author

Robert Sigler (code@sigler.io)