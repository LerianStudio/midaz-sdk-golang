package entities

import "net/url"

func pathSegment(value string) string {
	switch value {
	case ".":
		return "%2E"
	case "..":
		return "%2E%2E"
	}

	return url.PathEscape(value)
}
