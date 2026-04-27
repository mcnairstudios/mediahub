package client

import (
	"regexp"
	"sort"
	"strings"
)

type Detector struct {
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

func (d *Detector) Detect(port int, headers map[string]string) *Client {
	for i := range d.clients {
		if Match(d.clients[i], port, headers) {
			return &d.clients[i]
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
	case "exact":
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
