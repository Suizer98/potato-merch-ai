package storeagent

import (
	"context"
	"fmt"
	"log"
	"strings"

	adkagent "google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/agent/workflowagent"
	"google.golang.org/adk/v2/model"
	adksession "google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/workflow"
	"google.golang.org/genai"

	"github.com/Suizer98/potato-merch-ai/internal/config"
)

const (
	AppName          = "potato-merch"
	RootAgentName    = "potato"
	ShopAgentName    = "shop"
	BillingAgentName = "billing"
	SupportAgentName = "support"
	DefaultUserID    = "store"
)

func NewRootAgent(_ context.Context, cfg config.Config, llm model.LLM) (adkagent.Agent, error) {
	crm := newCRMClient(cfg.CRMURL, cfg.CRMOrigin, cfg.TwentyAPIKey, cfg.AdminEmail, cfg.AdminPassword)
	_, _, supportTools := mcpToolsets(cfg)

	support, err := llmagent.New(llmagent.Config{
		Name:        SupportAgentName,
		Model:       llm,
		Description: "Shipping, fit/care, restocks, and support tickets in Twenty CRM.",
		Mode:        llmagent.ModeSingleTurn,
		Instruction: supportInstruction,
		Toolsets:    supportTools,
	})
	if err != nil {
		return nil, fmt.Errorf("support agent: %w", err)
	}

	supportNode, err := workflow.NewAgentNode(support, workflow.NodeConfig{})
	if err != nil {
		return nil, fmt.Errorf("support node: %w", err)
	}

	classify := workflow.NewFunctionNode("classify", classifyTurn, workflow.NodeConfig{})
	shop := workflow.NewFunctionNode(ShopAgentName, func(ctx adkagent.Context, msg string) (*adksession.Event, error) {
		return shopTurn(ctx, msg, crm)
	}, workflow.NodeConfig{})
	billing := workflow.NewFunctionNode(BillingAgentName, func(ctx adkagent.Context, msg string) (*adksession.Event, error) {
		return billingTurn(ctx, msg, crm)
	}, workflow.NodeConfig{})
	greet := workflow.NewFunctionNode("greet", greetTurn, workflow.NodeConfig{})

	edges := workflow.Concat(
		workflow.Chain(workflow.Start, classify),
		[]workflow.Edge{
			{From: classify, To: shop, Route: workflow.StringRoute(ShopAgentName)},
			{From: classify, To: billing, Route: workflow.StringRoute(BillingAgentName)},
			{From: classify, To: supportNode, Route: workflow.StringRoute(SupportAgentName)},
			{From: classify, To: greet, Route: workflow.StringRoute(RootAgentName)},
		},
	)

	root, err := workflowagent.New(workflowagent.Config{
		Name:        RootAgentName,
		Description: "Potato Merch storefront concierge.",
		Edges:       edges,
	})
	if err != nil {
		return nil, fmt.Errorf("root workflow: %w", err)
	}

	log.Printf("adk graph ready (go router → shop/billing/support, crm=%v mcp=%v)", crm != nil, cfg.TwentyAPIKey != "")
	return root, nil
}

func classifyTurn(ctx adkagent.Context, msg string) (*adksession.Event, error) {
	text := strings.TrimSpace(msg)
	if text == "" {
		text = userText(ctx)
	}
	route := classifyRoute(text)
	if route == "" {
		route = stickyRoute(ctx)
	}
	if route == "" {
		route = RootAgentName
	}
	if err := ctx.Session().State().Set(routeStateKey, route); err != nil {
		log.Printf("route state: %v", err)
	}
	if number := firstOrderNumber(text, sessionUserText(ctx)); number != "" {
		_ = ctx.Session().State().Set(orderNumberKey, number)
	}

	event := adksession.NewEvent(ctx, ctx.InvocationID())
	event.Author = RootAgentName
	event.Routes = []string{route}
	event.Output = text
	return event, nil
}

func greetTurn(_ adkagent.Context, _ string) (string, error) {
	return "Hey — I can help with tees, orders, or shipping. What do you need?", nil
}

func shopTurn(ctx adkagent.Context, msg string, crm *crmClient) (*adksession.Event, error) {
	text := strings.TrimSpace(msg)
	if text == "" {
		text = userText(ctx)
	}
	reply := formatCatalogReply(crm, text)
	event := adksession.NewEvent(ctx, ctx.InvocationID())
	event.Author = ShopAgentName
	event.Output = reply
	event.Content = &genai.Content{
		Role:  genai.RoleModel,
		Parts: []*genai.Part{{Text: reply}},
	}
	return event, nil
}

func formatCatalogReply(crm *crmClient, text string) string {
	products, err := crm.listProducts()
	if err != nil {
		log.Printf("crm products: %v", err)
		return "I can’t see live inventory right now. Try the store catalog, or ask again in a moment."
	}
	if len(products) == 0 {
		return "The catalog is empty in CRM right now."
	}
	match := pickProduct(text, products)
	if match == nil {
		return "Which tee? We have " + productNames(products) + "."
	}
	return formatProduct(*match)
}

func pickProduct(text string, products []productRecord) *productRecord {
	query := strings.ToLower(text)
	bestIdx := -1
	bestScore := 0
	for i, product := range products {
		score := productScore(query, product)
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}
	if bestIdx < 0 || bestScore < 3 {
		return nil
	}
	return &products[bestIdx]
}

func productScore(query string, product productRecord) int {
	name := strings.ToLower(product.Name)
	sku := strings.ToLower(product.SKU)
	if name != "" && (strings.Contains(query, name) || strings.Contains(name, strings.TrimSpace(query))) {
		return 100
	}
	if sku != "" && strings.Contains(query, sku) {
		return 80
	}
	score := 0
	for _, token := range strings.FieldsFunc(name, func(r rune) bool {
		return r == ' ' || r == '-' || r == '_'
	}) {
		if len(token) < 3 {
			continue
		}
		if !strings.Contains(query, token) {
			continue
		}
		switch token {
		case "tee", "potato", "spud", "tater":
			score++
		default:
			score += 3
		}
	}
	return score
}

func productNames(products []productRecord) string {
	names := make([]string, 0, len(products))
	for _, product := range products {
		if product.Name != "" {
			names = append(names, product.Name)
		}
	}
	return strings.Join(names, ", ")
}

func formatProduct(product productRecord) string {
	status := catalogStatus(product)
	reply := product.Name + " is " + status + "."
	if stock := wholeNumber(product.Stock); stock != "" && !strings.EqualFold(product.Availability, "SOLD_OUT") && stock != "0" {
		reply += " " + stock + " left."
	}
	if product.Price != "" {
		reply += " " + product.Price + " SGD"
		if product.OnSale && product.CompareAt != "" {
			reply += " (was " + product.CompareAt + ")"
		}
		reply += "."
	}
	if product.Sizes != "" {
		reply += " Sizes " + product.Sizes + "."
	}
	return reply
}

func catalogStatus(product productRecord) string {
	switch strings.ToUpper(product.Availability) {
	case "SOLD_OUT":
		return "sold out"
	case "PREORDER":
		return "on preorder"
	default:
		if wholeNumber(product.Stock) == "0" {
			return "sold out"
		}
		return "in stock"
	}
}

func wholeNumber(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasSuffix(value, ".00") {
		return strings.TrimSuffix(value, ".00")
	}
	return value
}

func billingTurn(ctx adkagent.Context, msg string, crm *crmClient) (*adksession.Event, error) {
	text := strings.TrimSpace(msg)
	number := firstOrderNumber(text, sessionUserText(ctx))
	if number == "" {
		number = rememberedOrderNumber(ctx)
	}
	if number != "" {
		_ = ctx.Session().State().Set(orderNumberKey, number)
	}

	reply := "Sure. What’s the order number? It looks like ORD-1042."
	if number != "" {
		reply = formatOrderStatus(crm.findOrder(number))
	}

	event := adksession.NewEvent(ctx, ctx.InvocationID())
	event.Author = BillingAgentName
	event.Output = reply
	event.Content = &genai.Content{
		Role:  genai.RoleModel,
		Parts: []*genai.Part{{Text: reply}},
	}
	return event, nil
}

func formatOrderStatus(record orderRecord) string {
	if record.OrderNumber == "" {
		return "I need an order number like ORD-1042 to look it up."
	}
	if !record.Found {
		if record.Message != "" {
			return "I couldn’t find " + record.OrderNumber + " in CRM. " + record.Message
		}
		return "I couldn’t find " + record.OrderNumber + " in CRM. Check the number, or share the email on the order."
	}
	reply := "Order " + record.OrderNumber + " is " + record.Status + "."
	if record.Total != "" {
		reply += " Total " + record.Total + " SGD."
	}
	if record.CustomerEmail != "" {
		reply += " Email on the order: " + record.CustomerEmail + "."
	}
	return reply
}

func sessionUserText(ctx adkagent.Context) string {
	events := ctx.Session().Events()
	if events == nil {
		return ""
	}
	var parts []string
	for event := range events.All() {
		if event == nil || event.Content == nil || event.Content.Role != "user" {
			continue
		}
		var b strings.Builder
		for _, part := range event.Content.Parts {
			if part != nil {
				b.WriteString(part.Text)
			}
		}
		text := strings.TrimSpace(b.String())
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
}

func rememberedOrderNumber(ctx adkagent.Context) string {
	value, err := ctx.Session().State().Get(orderNumberKey)
	if err != nil {
		return ""
	}
	number, _ := value.(string)
	return firstOrderNumber(number)
}

func stickyRoute(ctx adkagent.Context) string {
	value, err := ctx.Session().State().Get(routeStateKey)
	if err != nil {
		return ""
	}
	route, _ := value.(string)
	if knownAgent(route) && route != RootAgentName {
		return route
	}
	return ""
}

func userText(ctx adkagent.Context) string {
	content := ctx.UserContent()
	if content == nil {
		return ""
	}
	var b strings.Builder
	for _, part := range content.Parts {
		if part != nil {
			b.WriteString(part.Text)
		}
	}
	return strings.TrimSpace(b.String())
}

func mcpToolsets(cfg config.Config) (shop, billing, support []tool.Toolset) {
	if cfg.TwentyMCPURL == "" || cfg.TwentyAPIKey == "" {
		log.Printf("mcp disabled (set TWENTY_MCP_URL and TWENTY_API_KEY)")
		return nil, nil, nil
	}
	shopSet, err := newMCPToolset(cfg.TwentyMCPURL, cfg.TwentyAPIKey, []string{"product", "sku", "catalog", "stock"}, false)
	if err != nil {
		log.Printf("shop mcp: %v", err)
	} else if shopSet != nil {
		shop = []tool.Toolset{shopSet}
	}
	billingSet, err := newMCPToolset(cfg.TwentyMCPURL, cfg.TwentyAPIKey, []string{"order", "customer", "invoice", "payment"}, false)
	if err != nil {
		log.Printf("billing mcp: %v", err)
	} else if billingSet != nil {
		billing = []tool.Toolset{billingSet}
	}
	supportSet, err := newMCPToolset(cfg.TwentyMCPURL, cfg.TwentyAPIKey, []string{"ticket", "support", "product"}, true)
	if err != nil {
		log.Printf("support mcp: %v", err)
	} else if supportSet != nil {
		support = []tool.Toolset{supportSet}
	}
	return shop, billing, support
}

const supportInstruction = `You are the Potato Merch support agent.
Help with shipping, wash/care, fit, and restocks. Use Product tools for catalog facts.
Create a Ticket in CRM only after the shopper confirms. Include customerEmail, category, and a short description.
Never change order totals.`
