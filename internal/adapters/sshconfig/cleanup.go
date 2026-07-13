package sshconfig

import (
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hmrdkn-labs/lazyss/internal/ports"
)

// Cleaner plans and applies cleanup for one SSH config path, implementing
// ports.CleanupPlanner.
type Cleaner struct {
	path string
}

// NewCleaner creates an SSH config cleanup adapter bound to a config path.
func NewCleaner(path string) Cleaner {
	return Cleaner{path: path}
}

type configHostBlock struct {
	hostBlock
	start int
	end   int
}

// PlanCleanup classifies concrete SSH Host blocks without mutating config.
func (c Cleaner) PlanCleanup(opts ports.CleanupOptions) (ports.CleanupPlan, error) {
	data, err := os.ReadFile(c.path)
	if err != nil {
		return ports.CleanupPlan{}, err
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 3 * time.Second
	}
	blocks := parseConfigHostBlocks(string(data))
	seenTargets := map[string]string{}
	items := make([]ports.CleanupItem, 0, len(blocks))
	for _, block := range blocks {
		if block.alias == "" || strings.ContainsAny(block.alias, "*?") {
			continue
		}
		item := ports.CleanupItem{
			Host:     block.alias,
			HostName: block.hostname,
			User:     block.user,
			Port:     nonzeroPort(block.port),
			Action:   ports.CleanupKeep,
			Reason:   "machine",
		}
		switch {
		case block.isSCMIdentity():
			item.Action = ports.CleanupKeep
			item.Reason = "scm identity"
			item.Protected = true
		case block.isPortForwardAlias():
			item.Action = ports.CleanupHide
			item.Reason = "port forward alias"
		default:
			key := cleanupTargetKey(block)
			if first, ok := seenTargets[key]; ok {
				item.Action = ports.CleanupDeleteCandidate
				item.Reason = "duplicate target"
				item.HostName = nonemptyString(item.HostName, first)
			} else {
				seenTargets[key] = block.hostname
			}
		}
		if opts.Check && item.HostName != "" {
			item.Check, item.CheckErr = tcpCheck(item.HostName, item.Port, opts.Timeout)
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Host < items[j].Host
	})
	return ports.CleanupPlan{Items: items}, nil
}

func tcpCheck(host string, port int, timeout time.Duration) (string, string) {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(nonzeroPort(port))), timeout)
	if err != nil {
		return "down", err.Error()
	}
	_ = conn.Close()
	return "up", ""
}

// ApplyCleanup removes selected non-protected Host blocks after creating a backup.
func (c Cleaner) ApplyCleanup(opts ports.CleanupApplyOptions) (ports.CleanupApplyResult, error) {
	if len(opts.Hosts) == 0 {
		return ports.CleanupApplyResult{}, fmt.Errorf("at least one --host is required with --write")
	}
	data, err := os.ReadFile(c.path)
	if err != nil {
		return ports.CleanupApplyResult{}, err
	}
	blocks := parseConfigHostBlocks(string(data))
	selected := map[string]bool{}
	for _, host := range opts.Hosts {
		selected[host] = true
	}
	found := map[string]configHostBlock{}
	for _, block := range blocks {
		if selected[block.alias] {
			found[block.alias] = block
		}
	}
	for host := range selected {
		block, ok := found[host]
		if !ok {
			return ports.CleanupApplyResult{}, fmt.Errorf("host %q not found", host)
		}
		if block.isSCMIdentity() {
			return ports.CleanupApplyResult{}, fmt.Errorf("host %q is protected scm identity", host)
		}
	}

	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	backupPath := fmt.Sprintf("%s.lazyss-backup-%s", c.path, now.Format("20060102-150405"))
	if err := os.WriteFile(backupPath, data, 0o600); err != nil {
		return ports.CleanupApplyResult{}, err
	}

	lines := splitLinesPreserve(string(data))
	removeLines := map[int]bool{}
	for _, block := range found {
		for i := block.start; i < block.end; i++ {
			removeLines[i] = true
		}
	}
	var out strings.Builder
	for i, line := range lines {
		if !removeLines[i] {
			out.WriteString(line)
		}
	}
	mode := os.FileMode(0o600)
	if info, err := os.Stat(c.path); err == nil {
		mode = info.Mode().Perm()
	}
	if err := os.WriteFile(c.path, []byte(out.String()), mode); err != nil {
		return ports.CleanupApplyResult{}, err
	}
	removed := make([]string, 0, len(found))
	for host := range found {
		removed = append(removed, host)
	}
	sort.Strings(removed)
	return ports.CleanupApplyResult{BackupPath: backupPath, RemovedHosts: removed}, nil
}

func parseConfigHostBlocks(data string) []configHostBlock {
	lines := splitLinesPreserve(data)
	var blocks []configHostBlock
	current := -1
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.ToLower(fields[0])
		value := strings.Join(fields[1:], " ")
		if key == "host" {
			if current >= 0 {
				blocks[current].end = i
			}
			blocks = append(blocks, configHostBlock{hostBlock: hostBlock{alias: fields[1]}, start: i, end: len(lines)})
			current = len(blocks) - 1
			continue
		}
		if current < 0 {
			continue
		}
		switch key {
		case "hostname":
			blocks[current].hostname = value
		case "user":
			blocks[current].user = value
		case "port":
			if p, err := strconv.Atoi(value); err == nil {
				blocks[current].port = p
			}
		case "localforward":
			blocks[current].localForwards = append(blocks[current].localForwards, value)
		case "identityfile":
			blocks[current].identityFiles = append(blocks[current].identityFiles, value)
		case "proxycommand":
			blocks[current].proxyCommand = value
		}
	}
	return blocks
}

func splitLinesPreserve(data string) []string {
	if data == "" {
		return nil
	}
	lines := strings.SplitAfter(data, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func cleanupTargetKey(block configHostBlock) string {
	return strings.ToLower(block.user) + "|" + strings.ToLower(nonemptyString(block.hostname, block.alias)) + "|" + strconv.Itoa(nonzeroPort(block.port))
}

func nonzeroPort(port int) int {
	if port == 0 {
		return 22
	}
	return port
}

func nonemptyString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
