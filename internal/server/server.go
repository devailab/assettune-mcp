package server

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/devailab/assettune-mcp/internal/tools/image"
	"github.com/devailab/assettune-mcp/internal/tools/video"
)

var (
	Name    = "assettune-mcp"
	Version = "v0.1.0"
)

func New() *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    Name,
		Version: Version,
	}, nil)
	image.Register(server)
	video.Register(server)
	return server
}
