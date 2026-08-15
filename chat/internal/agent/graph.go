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
	shopTools, _, supportTools := mcpToolsets(cfg)

	shop, err := llmagent.New(llmagent.Config{
		Name:        ShopAgentName,
		Model:       llm,
		Description: "Catalog specialist for Potato Merch tees, sizes, stock, seasons, and sales.",
		Mode:        llmagent.ModeSingleTurn,
		Instruction: shopInstruction,
		Toolsets:    shopTools,
	})
	if err != nil {
		return nil, fmt.Errorf("shop agent: %w", err)
	}

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

	shopNode, err := workflow.NewAgentNode(shop, workflow.NodeConfig{})
	if err != nil {
		return nil, fmt.Errorf("shop node: %w", err)
	}
	supportNode, err := workflow.NewAgentNode(support, workflow.NodeConfig{})
	if err != nil {
		return nil, fmt.Errorf("support node: %w", err)
	}

	classify := workflow.NewFunctionNode("classify", classifyTurn, workflow.NodeConfig{})
	billing := workflow.NewFunctionNode(BillingAgentName, func(ctx adkagent.Context, msg string) (*adksession.Event, error) {
		return billingTurn(ctx, msg, crm)
	}, workflow.NodeConfig{})
	greet := workflow.NewFunctionNode("greet", greetTurn, workflow.NodeConfig{})

	edges := workflow.Concat(
		workflow.Chain(workflow.Start, classify),
		[]workflow.Edge{
			{From: classify, To: shopNode, Route: workflow.StringRoute(ShopAgentName)},
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

const shopInstruction = `You are the Potato Merch shop agent.
Answer catalog questions from CRM Product records when tools are available: name, sku, price (SGD), sizes, season, availability, stock.
If tools fail, say you cannot see live inventory. Never invent restocks. Never change prices or stock.`

const supportInstruction = `You are the Potato Merch support agent.
Help with shipping, wash/care, fit, and restocks. Use Product tools for catalog facts.
Create a Ticket in CRM only after the shopper confirms. Include customerEmail, category, and a short description.
Never change order totals.`
