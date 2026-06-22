package prompts

import "regexp"

var sanitizePattern = regexp.MustCompile(`[\x00-\x1F\x7F-\x9F]`)

func SanitizeInput(input string) string {
	return sanitizePattern.ReplaceAllString(input, "")
}
