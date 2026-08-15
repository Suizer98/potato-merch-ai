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

type productRecord struct {
	Found        bool
	Name         string
	SKU          string
	Price        string
	CompareAt    string
	Stock        string
	Availability string
	Sizes        string
	Season       string
	OnSale       bool
	Message      string
}

type orderRecord struct {
	Found         bool   `json:"found"`
	OrderNumber   string `json:"order_number,omitempty"`
	Status        string `json:"status,omitempty"`
	Total         string `json:"total,omitempty"`
	CustomerEmail string `json:"customer_email,omitempty"`
	Message       string `json:"message,omitempty"`
}

func (c *crmClient) listProducts() ([]productRecord, error) {
	if c == nil {
		return nil, fmt.Errorf("CRM is not configured")
	}
	token, err := c.accessToken()
	if err != nil {
		return nil, err
	}
	queries := []string{
		`query { products(first: 100) { edges { node { id name sku price compareAtPrice stock availability sizes season isOnSale } } } }`,
		`query { products(paging:{first:100}) { edges { node { id name sku price compareAtPrice stock availability sizes season isOnSale } } } }`,
	}
	var lastErr error
	for _, query := range queries {
		data, err := c.graphql("/graphql", query, nil, token)
		if err != nil {
			lastErr = err
			continue
		}
		nodes := connectionNodes(data, "products")
		if len(nodes) == 0 {
			continue
		}
		out := make([]productRecord, 0, len(nodes))
		for _, node := range nodes {
			out = append(out, productFromNode(node))
		}
		return out, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, nil
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
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Since(c.tokenAt) < 8*time.Minute {
		return c.token, nil
	}
	if c.apiKey != "" {
		if err := c.ping(c.apiKey); err == nil {
			c.token = c.apiKey
			c.tokenAt = time.Now()
			return c.apiKey, nil
		} else {
			log.Printf("crm api key rejected (%v); falling back to admin sign-in", err)
			c.apiKey = ""
		}
	}
	return c.signIn()
}

func (c *crmClient) ping(token string) error {
	_, err := c.graphql("/graphql", "query { orders(paging:{first:1}){ edges { node { id } } } }", nil, token)
	return err
}

func (c *crmClient) signIn() (string, error) {
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

func productFromNode(node map[string]any) productRecord {
	return productRecord{
		Found:        true,
		Name:         stringField(node, "name", ""),
		SKU:          stringField(node, "sku", ""),
		Price:        stringField(node, "price", ""),
		CompareAt:    stringField(node, "compareAtPrice", ""),
		Stock:        stringField(node, "stock", ""),
		Availability: stringField(node, "availability", ""),
		Sizes:        stringField(node, "sizes", ""),
		Season:       stringField(node, "season", ""),
		OnSale:       boolField(node, "isOnSale"),
	}
}

func firstOrderNode(data map[string]any) map[string]any {
	nodes := connectionNodes(data, "orders")
	if len(nodes) == 0 {
		return nil
	}
	return nodes[0]
}

func connectionNodes(data map[string]any, key string) []map[string]any {
	conn, _ := data[key].(map[string]any)
	if conn == nil {
		return nil
	}
	edges, _ := conn["edges"].([]any)
	out := make([]map[string]any, 0, len(edges))
	for _, item := range edges {
		edge, _ := item.(map[string]any)
		node, _ := edge["node"].(map[string]any)
		if node != nil {
			out = append(out, node)
		}
	}
	return out
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

func boolField(obj map[string]any, key string) bool {
	if obj == nil {
		return false
	}
	switch typed := obj[key].(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(typed, "true")
	}
	return false
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
