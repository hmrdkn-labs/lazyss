package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hamardikan/lazyss/internal/domain"
)

type cockpitFilter struct {
	Raw        string
	Tags       map[string]string
	NamePrefix string
	Method     string
	Health     string
	Text       string
}

func parseFilterExpression(raw string) (cockpitFilter, error) {
	filter := cockpitFilter{Raw: strings.TrimSpace(raw), Tags: map[string]string{}}
	var text []string
	for _, token := range strings.Fields(raw) {
		key, value, ok := splitFilterToken(token)
		if !ok {
			text = append(text, token)
			continue
		}
		switch strings.ToLower(key) {
		case "name":
			filter.NamePrefix = value
		case "method":
			method, err := normalizeMethodFilter(value)
			if err != nil {
				return cockpitFilter{}, err
			}
			filter.Method = method
		case "health":
			health, err := normalizeHealthFilter(value)
			if err != nil {
				return cockpitFilter{}, err
			}
			filter.Health = health
		default:
			tagKey := key
			if strings.HasPrefix(strings.ToLower(tagKey), "tag:") {
				tagKey = tagKey[4:]
			}
			if tagKey == "" {
				return cockpitFilter{}, fmt.Errorf("empty tag key in %q", token)
			}
			filter.Tags[tagKey] = value
		}
	}
	filter.Text = strings.Join(text, " ")
	return filter, nil
}

func splitFilterToken(token string) (string, string, bool) {
	if strings.HasPrefix(strings.ToLower(token), "tag:") {
		rest := token[4:]
		if idx := strings.Index(rest, "="); idx >= 0 {
			return "tag:" + rest[:idx], rest[idx+1:], true
		}
		return "", "", false
	}
	if idx := strings.Index(token, "="); idx >= 0 {
		return token[:idx], token[idx+1:], true
	}
	if idx := strings.Index(token, ":"); idx >= 0 {
		return token[:idx], token[idx+1:], true
	}
	return "", "", false
}

func normalizeMethodFilter(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "ssh":
		return string(domain.AccessSSH), nil
	case "ssm", "aws", "aws-ssm", "aws-ssm-shell":
		return string(domain.AccessAWSSSMShell), nil
	default:
		return "", fmt.Errorf("unknown method filter %q", value)
	}
}

func normalizeHealthFilter(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case string(domain.HealthUp), string(domain.HealthDown), string(domain.HealthUnknown):
		return value, nil
	default:
		return "", fmt.Errorf("unknown health filter %q", value)
	}
}

func (f cockpitFilter) empty() bool {
	return f.Raw == "" && len(f.Tags) == 0 && f.NamePrefix == "" && f.Method == "" && f.Health == "" && f.Text == ""
}

func (f cockpitFilter) queryTags() map[string]string {
	if len(f.Tags) == 0 {
		return nil
	}
	out := make(map[string]string, len(f.Tags))
	for key, value := range f.Tags {
		out[key] = value
	}
	return out
}

func (f cockpitFilter) matches(machine domain.Machine) bool {
	if f.NamePrefix != "" && !strings.HasPrefix(strings.ToLower(machine.Name), strings.ToLower(f.NamePrefix)) {
		return false
	}
	if f.Method != "" && !machineHasMethod(machine, domain.AccessMethod(f.Method)) {
		return false
	}
	if f.Health != "" && strings.ToLower(string(machine.Health.Status)) != f.Health {
		return false
	}
	for key, value := range f.Tags {
		if machine.ProviderTags[key] != value {
			return false
		}
	}
	if f.Text != "" && !strings.Contains(machineSearchText(machine), strings.ToLower(f.Text)) {
		return false
	}
	return true
}

func machineHasMethod(machine domain.Machine, method domain.AccessMethod) bool {
	for _, candidate := range machine.Methods {
		if candidate == method {
			return true
		}
	}
	return false
}

func machineSearchText(machine domain.Machine) string {
	parts := []string{
		machine.Name,
		machine.Address,
		machine.NativeID,
		string(machine.Provider),
		string(machine.DefaultMethod()),
		machine.Health.Label,
	}
	if len(machine.ProviderTags) > 0 {
		keys := make([]string, 0, len(machine.ProviderTags))
		for key := range machine.ProviderTags {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			parts = append(parts, key+"="+machine.ProviderTags[key])
		}
	}
	return strings.ToLower(strings.Join(parts, " "))
}

func sortedProviderTags(tags map[string]string) []string {
	keys := make([]string, 0, len(tags))
	for key := range tags {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, key+"="+tags[key])
	}
	return out
}
