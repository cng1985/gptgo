package skill

import "regexp"

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)
var mcpLike = regexp.MustCompile(`^m[0-9]+n[0-9]+_`)

func validName(name string) bool {
	if !namePattern.MatchString(name) {
		return false
	}
	switch name {
	case "current_time", "calculator":
		return false
	}
	return !mcpLike.MatchString(name)
}
