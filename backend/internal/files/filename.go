package files

import (
	"errors"
	"strings"
	"unicode"
)

type SanitizedFilename struct {
	Original string
	Stored   string
}

func SanitizeFilename(input string) (SanitizedFilename, error) {
	original := input
	normalized := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, input)
	normalized = strings.ReplaceAll(normalized, "\\", "/")

	parts := strings.Split(normalized, "/")
	flattened := make([]string, 0, len(parts))
	for _, part := range parts {
		part = collapseWhitespace(strings.TrimSpace(part))
		switch part {
		case "", ".", "..":
			continue
		default:
			flattened = append(flattened, part)
		}
	}

	stored := strings.Join(flattened, "-")
	stored = collapseWhitespace(strings.TrimSpace(stored))
	stored = strings.TrimLeft(stored, ".")
	stored = strings.TrimRight(stored, ". ")

	if stored == "" || strings.Trim(stored, ".") == "" {
		return SanitizedFilename{}, errors.New("filename cannot be empty after sanitization")
	}

	return SanitizedFilename{
		Original: original,
		Stored:   stored,
	}, nil
}

func collapseWhitespace(input string) string {
	if input == "" {
		return ""
	}

	return strings.Join(strings.Fields(input), " ")
}
