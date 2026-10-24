package toolkit

import (
	"fmt"
	"strconv"
	"strings"
)

func publicMCPName(serverID int64, index int, remote string) string {
	suffix := sanitizeToken(remote)
	if suffix == "" {
		suffix = "tool"
	}
	prefix := fmt.Sprintf("m%dn%d_", serverID, index)
	room := 64 - len(prefix)
	if room < 1 {
		if len(prefix) > 64 {
			return prefix[:64]
		}
		return prefix
	}
	if len(suffix) > room {
		suffix = suffix[:room]
	}
	return prefix + suffix
}

func mcpServerID(name string) (int64, bool) {
	if len(name) < 5 || name[0] != 'm' {
		return 0, false
	}
	i := 1
	for i < len(name) && name[i] >= '0' && name[i] <= '9' {
		i++
	}
	if i == 1 || i >= len(name) || name[i] != 'n' {
		return 0, false
	}
	j := i + 1
	if j >= len(name) || name[j] < '0' || name[j] > '9' {
		return 0, false
	}
	for j < len(name) && name[j] >= '0' && name[j] <= '9' {
		j++
	}
	if j >= len(name) || name[j] != '_' {
		return 0, false
	}
	id, err := strconv.ParseInt(name[1:i], 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

func sanitizeToken(value string) string {
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
		} else if r == ' ' || r == '.' || r == '/' {
			b.WriteByte('_')
		}
	}
	return strings.Trim(b.String(), "_")
}

func reservedToolName(name string) bool {
	switch name {
	case "current_time", "calculator":
		return true
	}
	_, ok := mcpServerID(name)
	return ok
}
