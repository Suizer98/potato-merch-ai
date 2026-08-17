package storeagent

import (
	"regexp"
	"strings"
)

var (
	orderNumberPattern = regexp.MustCompile(`(?i)\bord-[a-z0-9]+\b`)
	emailPattern       = regexp.MustCompile(`(?i)\b[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}\b`)
)

func classifyRoute(text string) string {
	// Support before billing so "shipping status" / "wrong size order" don't stick on billing.
	if looksLikeSupport(text) {
		return SupportAgentName
	}
	if looksLikeBilling(text) {
		return BillingAgentName
	}
	if looksLikeCatalog(text) {
		return ShopAgentName
	}
	return ""
}

// wantsTopicSwitch is true when the user is leaving the current specialist
// without naming a new one (clears sticky → greet).
func wantsTopicSwitch(text string) bool {
	lower := strings.ToLower(text)
	for _, key := range topicSwitchPhrases {
		if strings.Contains(lower, key) {
			return true
		}
	}
	return false
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
	for _, key := range catalogKeywords {
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
	for _, key := range billingKeywords {
		if strings.Contains(lower, key) {
			return true
		}
	}
	if strings.Contains(lower, "order") {
		for _, key := range billingOrderHints {
			if strings.Contains(lower, key) {
				return true
			}
		}
	}
	return false
}

func looksLikeSupport(text string) bool {
	lower := strings.ToLower(text)
	for _, key := range supportKeywords {
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
