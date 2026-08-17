package storeagent

const (
	AppName          = "potato-merch"
	RootAgentName    = "potato"
	ShopAgentName    = "shop"
	BillingAgentName = "billing"
	SupportAgentName = "support"
	DefaultUserID    = "store"

	routeStateKey  = "route"
	orderNumberKey = "orderNumber"

	RootDescription    = "Potato Merch storefront concierge."
	SupportDescription = "Shipping, fit/care, restocks, and support tickets in Twenty CRM."

	GreetReply            = "Hey — I can help with tees, orders, or shipping. What do you need?"
	CatalogUnavailable    = "I can’t see live inventory right now. Try the store catalog, or ask again in a moment."
	CatalogEmpty          = "The catalog is empty in CRM right now."
	CatalogWhichTee       = "Which tee? We have "
	BillingAskOrder       = "Sure. What’s the order number? It looks like ORD-1042."
	BillingNeedOrder      = "I need an order number like ORD-1042 to look it up."
	BillingNotFoundHint   = "Check the number, or share the email on the order."
	StatusCheckingShop    = "Checking the catalog…"
	StatusCheckingBilling = "Looking up your order…"
	StatusCheckingSupport = "Checking with support…"

	StatusSoldOut  = "sold out"
	StatusPreorder = "on preorder"
	StatusInStock  = "in stock"

	routerInstruction = `Choose the best agent for the user's latest message.

Allowed routes:
- shop: products, catalog, sizes, stock, sales, and merchandise
- billing: orders, payments, invoices, refunds, and order tracking
- support: shipping problems, damaged or missing items, care, fit, restocks, and support tickets
- potato: greetings, general concierge questions, reset requests, or anything outside these areas

If the message is a short follow-up, choose the current agent when appropriate.
Return exactly one word: shop, billing, support, or potato.`

	supportInstruction = `You are the support agent for Potato Merch, an online shop selling potato-themed tees.
You are talking to a shopper. Help with shipping, wash/care, fit, restocks, and support tickets.
Use Product tools for catalog facts. Create a Ticket only after the shopper confirms,
including customerEmail, category, and a short description. Never change order totals.

Twenty is only the internal CRM used to store records. Never mention Twenty, your tools,
workspaces, metadata, skills, or documentation to the shopper, and never offer help with
CRM or platform software.

When asked what you can do, reply in 3-5 short bullets about shopper topics only
(shipping and delivery, sizing and fit, wash and care, restocks, raising a support ticket).
Keep answers short and plain; no tables.`
)

var (
	topicSwitchPhrases = []string{
		"something else",
		"another question",
		"different question",
		"never mind",
		"nevermind",
		"new question",
		"talk to someone else",
		"start over",
	}
	catalogKeywords = []string{
		"tee", "size", "stock", "season", "sale", "sku", "catalog", "merch", "potato", "spud", "tater",
	}
	billingKeywords = []string{
		"refund", "paid", "invoice", "payment", "where is my order", "order status", "order number",
	}
	billingOrderHints = []string{
		"track", "shipped", "delivery", "receipt", "charged", "total",
	}
	supportKeywords = []string{
		"ticket", "shipping", "wrong size", "restock", "wash", "fit", "care", "damaged", "missing",
		"support", "what can you do", "what can you help", "how can you help", "actions you can",
	}
	// Twenty platform meta tools leak CRM-vendor docs into shopper answers.
	deniedToolKeywords = []string{
		"help_center", "help center", "help centre", "skill", "metadata", "documentation",
	}
)

func catalogAskWhich(names string) string {
	return CatalogWhichTee + names + "."
}

func billingNotFound(orderNumber, detail string) string {
	if detail != "" {
		return "I couldn’t find " + orderNumber + " in CRM. " + detail
	}
	return "I couldn’t find " + orderNumber + " in CRM. " + BillingNotFoundHint
}

func billingOrderStatus(orderNumber, status, total, email string) string {
	reply := "Order " + orderNumber + " is " + status + "."
	if total != "" {
		reply += " Total " + total + " USD."
	}
	if email != "" {
		reply += " Email on the order: " + email + "."
	}
	return reply
}
