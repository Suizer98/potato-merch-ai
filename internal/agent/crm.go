package storeagent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type crmClient struct {
	baseURL  string
	origin   string
	apiKey   string
	email    string
	password string
	http     *http.Client
	mu       sync.Mutex
	token    string
	tokenAt  time.Time
}

func newCRMClient(baseURL, origin, apiKey, email, password string) *crmClient {
	if strings.TrimSpace(baseURL) == "" {
		return nil
	}
	return &crmClient{
		baseURL:  strings.TrimRight(baseURL, "/"),
		origin:   origin,
		apiKey:   strings.TrimSpace(apiKey),
		email:    email,
		password: password,
		http:     &http.Client{Timeout: 20 * time.Second},
	}
}

type orderRecord struct {
	Found         bool   `json:"found"`
	OrderNumber   string `json:"order_number,omitempty"`
	Status        string `json:"status,omitempty"`
	Total         string `json:"total,omitempty"`
	CustomerEmail string `json:"customer_email,omitempty"`
	Message       string `json:"message,omitempty"`
}

func (c *crmClient) findOrder(orderNumber string) orderRecord {
	orderNumber = strings.TrimSpace(orderNumber)
	if c == nil {
		return orderRecord{Found: false, OrderNumber: orderNumber, Message: "CRM is not configured."}
	}
	if orderNumber == "" {
		return orderRecord{Found: false, Message: "No order number was provided."}
	}

	token, err := c.accessToken()
	if err != nil {
		return orderRecord{Found: false, OrderNumber: orderNumber, Message: "Could not sign in to CRM: " + err.Error()}
	}

	queries := []string{
		`query($n:String!){ orders(filter:{orderNumber:{eq:$n}}){ edges { node { id name orderNumber total status customerEmail } } } }`,
		`query($n:String!){ orders(filter:{name:{eq:$n}}){ edges { node { id name orderNumber total status customerEmail } } } }`,
	}
	var lastErr error
	sawEmpty := false
	for _, query := range queries {
		data, err := c.graphql("/graphql", query, map[string]any{"n": orderNumber}, token)
		if err != nil {
			lastErr = err
			continue
		}
		node := firstOrderNode(data)
		if node == nil {
			sawEmpty = true
			continue
		}
		return orderRecord{
			Found:         true,
			OrderNumber:   stringField(node, "orderNumber", stringField(node, "name", orderNumber)),
			Status:        stringField(node, "status", ""),
			Total:         stringField(node, "total", ""),
			CustomerEmail: stringField(node, "customerEmail", ""),
		}
	}
	if sawEmpty {
		return orderRecord{Found: false, OrderNumber: orderNumber, Message: "No order with that number in CRM."}
	}
	if lastErr != nil {
		log.Printf("crm order %s: %v", orderNumber, lastErr)
		return orderRecord{Found: false, OrderNumber: orderNumber, Message: "Could not query CRM."}
	}
	return orderRecord{Found: false, OrderNumber: orderNumber, Message: "No order with that number in CRM."}
}

func (c *crmClient) accessToken() (string, error) {
	if c.apiKey != "" {
		return c.apiKey, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Since(c.tokenAt) < 8*time.Minute {
		return c.token, nil
	}
	if c.email == "" || c.password == "" {
		return "", fmt.Errorf("ADMIN_EMAIL and ADMIN_PASSWORD are required")
	}

	signIn, err := c.graphql(
		"/metadata",
		`mutation($e:String!,$p:String!){
			signIn(email:$e,password:$p){
				availableWorkspaces { availableWorkspacesForSignIn { id loginToken } }
			}
		}`,
		map[string]any{"e": c.email, "p": c.password},
		"",
	)
	if err != nil {
		return "", err
	}
	workspaces := nestedMap(signIn, "signIn", "availableWorkspaces", "availableWorkspacesForSignIn")
	if len(workspaces) == 0 {
		return "", fmt.Errorf("CRM signIn returned no workspaces")
	}
	loginToken := stringField(workspaces[0], "loginToken", "")
	if loginToken == "" {
		return "", fmt.Errorf("CRM signIn returned no login token")
	}

	exchange, err := c.graphql(
		"/metadata",
		`mutation($t:String!,$o:String!){
			getAuthTokensFromLoginToken(loginToken:$t,origin:$o){
				tokens { accessOrWorkspaceAgnosticToken { token } }
			}
		}`,
		map[string]any{"t": loginToken, "o": c.origin},
		"",
	)
	if err != nil {
		return "", err
	}
	token := stringField(nestedObject(exchange, "getAuthTokensFromLoginToken", "tokens", "accessOrWorkspaceAgnosticToken"), "token", "")
	if token == "" {
		return "", fmt.Errorf("CRM token exchange returned no token")
	}
	c.token = token
	c.tokenAt = time.Now()
	return token, nil
}

func (c *crmClient) graphql(path, query string, variables map[string]any, token string) (map[string]any, error) {
	payload := map[string]any{"query": query}
	if variables != nil {
		payload["variables"] = variables
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", c.origin)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var parsed struct {
		Data   map[string]any `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("CRM HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if len(parsed.Errors) > 0 {
		return nil, fmt.Errorf("%s", parsed.Errors[0].Message)
	}
	if parsed.Data == nil {
		return map[string]any{}, nil
	}
	return parsed.Data, nil
}

func firstOrderNode(data map[string]any) map[string]any {
	orders, _ := data["orders"].(map[string]any)
	if orders == nil {
		return nil
	}
	edges, _ := orders["edges"].([]any)
	if len(edges) == 0 {
		return nil
	}
	edge, _ := edges[0].(map[string]any)
	node, _ := edge["node"].(map[string]any)
	return node
}

func nestedObject(data map[string]any, keys ...string) map[string]any {
	current := data
	for _, key := range keys {
		if current == nil {
			return nil
		}
		next, _ := current[key].(map[string]any)
		current = next
	}
	return current
}

func nestedMap(data map[string]any, keys ...string) []map[string]any {
	if len(keys) == 0 {
		return nil
	}
	parentKeys := keys[:len(keys)-1]
	last := keys[len(keys)-1]
	parent := nestedObject(data, parentKeys...)
	if parent == nil {
		parent = data
	}
	raw, _ := parent[last].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if obj, ok := item.(map[string]any); ok {
			out = append(out, obj)
		}
	}
	return out
}

func stringField(obj map[string]any, key, fallback string) string {
	if obj == nil {
		return fallback
	}
	value := obj[key]
	switch typed := value.(type) {
	case nil:
		return fallback
	case string:
		if typed == "" {
			return fallback
		}
		return typed
	case float64:
		if typed == float64(int64(typed)) {
			return fmt.Sprintf("%.2f", typed)
		}
		return fmt.Sprintf("%g", typed)
	case json.Number:
		return typed.String()
	case map[string]any:
		if inner, ok := typed["value"].(string); ok && inner != "" {
			return inner
		}
		if inner, ok := typed["token"].(string); ok && inner != "" {
			return inner
		}
	}
	encoded, _ := json.Marshal(value)
	text := strings.Trim(string(encoded), `"`)
	if text == "" || text == "null" {
		return fallback
	}
	return text
}
