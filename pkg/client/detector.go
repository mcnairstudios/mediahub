package client

import (
	"context"
	"regexp"
	"sort"
	"strings"
)

type Detector struct {
	store   Store
	clients []Client
}

func NewDetector(clients []Client) *Detector {
	sorted := make([]Client, len(clients))
	copy(sorted, clients)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Priority > sorted[j].Priority
	})
	return &Detector{clients: sorted}
}

func NewDetectorWithStore(store Store) *Detector {
	return &Detector{store: store}
}

func (d *Detector) Detect(port int, headers map[string]string) *Client {
	clients := d.clients
	if d.store != nil {
		if live, err := d.store.List(context.Background()); err == nil {
			clients = live
		}
	}
	for i := range clients {
		if Match(clients[i], port, headers) {
			return &clients[i]
		}
	}
	return nil
}

func Match(c Client, port int, headers map[string]string) bool {
	if !c.IsEnabled {
		return false
	}

	if c.ListenPort != 0 && c.ListenPort != port {
		return false
	}

	for _, rule := range c.MatchRules {
		headerVal, ok := headers[rule.HeaderName]
		if rule.MatchType == "exists" {
			if !ok {
				return false
			}
			continue
		}
		if !ok {
			return false
		}
		if !matchRule(rule, headerVal) {
			return false
		}
	}

	return true
}

func matchRule(rule MatchRule, value string) bool {
	switch rule.MatchType {
	case "contains":
		return strings.Contains(value, rule.MatchValue)
	case "prefix":
		return strings.HasPrefix(value, rule.MatchValue)
	case "exact", "equals":
		return value == rule.MatchValue
	case "regex":
		re, err := regexp.Compile(rule.MatchValue)
		if err != nil {
			return false
		}
		return re.MatchString(value)
	default:
		return false
	}
}
