package storeagent

import (
	"regexp"
	"strings"
)

const (
	routeStateKey  = "route"
	orderNumberKey = "orderNumber"
)

var (
	orderNumberPattern = regexp.MustCompile(`(?i)\bord-[a-z0-9]+\b`)
	emailPattern       = regexp.MustCompile(`(?i)\b[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}\b`)
)

func classifyRoute(text string) string {
	if looksLikeBilling(text) {
		return BillingAgentName
	}
	if looksLikeSupport(text) {
		return SupportAgentName
	}
	if looksLikeCatalog(text) {
		return ShopAgentName
	}
	return ""
}

func HasOrderNumber(text string) bool {
	return firstOrderNumber(text) != ""
}

func firstOrderNumber(texts ...string) string {
	for _, text := range texts {
		match := orderNumberPattern.FindString(text)
		if match != "" {
			return strings.ToUpper(match)
		}
	}
	return ""
}

func looksLikeCatalog(text string) bool {
	lower := strings.ToLower(text)
	for _, key := range []string{"tee", "size", "stock", "season", "sale", "sku", "catalog", "merch", "potato", "spud", "tater"} {
		if strings.Contains(lower, key) {
			return true
		}
	}
	return false
}

func looksLikeBilling(text string) bool {
	if orderNumberPattern.MatchString(text) || emailPattern.MatchString(text) {
		return true
	}
	lower := strings.ToLower(text)
	for _, key := range []string{"order", "refund", "paid", "invoice", "payment", "status"} {
		if strings.Contains(lower, key) {
			return true
		}
	}
	return false
}

func looksLikeSupport(text string) bool {
	lower := strings.ToLower(text)
	for _, key := range []string{"ticket", "shipping", "wrong size", "restock", "wash"} {
		if strings.Contains(lower, key) {
			return true
		}
	}
	return false
}

func canonicalizeAgentName(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	lower = strings.ReplaceAll(lower, "-", "_")
	switch {
	case lower == ShopAgentName || strings.Contains(lower, "catalog") || strings.Contains(lower, "shop"):
		return ShopAgentName
	case lower == BillingAgentName || strings.Contains(lower, "order") || strings.Contains(lower, "billing") || strings.Contains(lower, "refund") || strings.Contains(lower, "invoice"):
		return BillingAgentName
	case lower == SupportAgentName || strings.Contains(lower, "ticket") || strings.Contains(lower, "shipping") || strings.Contains(lower, "support"):
		return SupportAgentName
	case lower == RootAgentName || lower == "concierge":
		return RootAgentName
	default:
		return ""
	}
}

func knownAgent(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case RootAgentName, ShopAgentName, BillingAgentName, SupportAgentName:
		return true
	default:
		return false
	}
}
