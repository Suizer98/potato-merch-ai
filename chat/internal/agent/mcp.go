package storeagent

import (
	"log"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	adkagent "google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/mcptoolset"
)

func newMCPToolset(endpoint, apiKey string, allow []string, writes bool) (tool.Toolset, error) {
	if endpoint == "" {
		return nil, nil
	}

	transport := &mcp.StreamableClientTransport{Endpoint: endpoint}
	if apiKey != "" {
		transport.HTTPClient = &http.Client{Transport: bearerTransport{token: apiKey}}
	}

	base, err := mcptoolset.New(mcptoolset.Config{
		Transport: transport,
		RequireConfirmationProvider: func(name string, _ any) bool {
			if !writes {
				return false
			}
			return isWriteTool(name, "")
		},
	})
	if err != nil {
		return nil, err
	}

	filtered := tool.FilterToolset(base, func(_ adkagent.ReadonlyContext, t tool.Tool) bool {
		blob := strings.ToLower(t.Name() + " " + t.Description())
		if isDeniedTool(blob) {
			return false
		}
		if !writes && isWriteTool(t.Name(), t.Description()) {
			return false
		}
		if len(allow) == 0 {
			return true
		}
		for _, key := range allow {
			if strings.Contains(blob, key) {
				return true
			}
		}
		return isGenericCRMTool(blob)
	})
	log.Printf("mcp toolset ready endpoint=%s writes=%v allow=%s", endpoint, writes, strings.Join(allow, ","))
	return filtered, nil
}

type bearerTransport struct {
	token string
	base  http.RoundTripper
}

func (t bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.Header.Set("Authorization", "Bearer "+t.token)
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(clone)
}

func isWriteTool(name, description string) bool {
	blob := strings.ToLower(name + " " + description)
	for _, key := range []string{"create", "update", "delete", "upsert", "insert", "write", "mutate", "remove"} {
		if strings.Contains(blob, key) {
			return true
		}
	}
	return false
}

func isDeniedTool(blob string) bool {
	for _, key := range deniedToolKeywords {
		if strings.Contains(blob, key) {
			return true
		}
	}
	return false
}

func isGenericCRMTool(blob string) bool {
	for _, key := range []string{"record", "object", "graphql", "search", "find", "query", "list"} {
		if strings.Contains(blob, key) {
			return true
		}
	}
	return false
}
