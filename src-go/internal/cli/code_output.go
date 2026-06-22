package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

var fencedCodeBlockPattern = regexp.MustCompile("(?s)```[^\\n`]*\\n(.*?)```")

func extractSingleCodeBlock(text string) (string, error) {
	matches := fencedCodeBlockPattern.FindAllStringSubmatch(text, -1)
	if len(matches) != 1 {
		return "", fmt.Errorf("expected exactly one fenced code block, found %d", len(matches))
	}
	return matches[0][1], nil
}

func writeCodeBlock(targetPath string, content string) error {
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(targetPath, []byte(content), 0o644)
}
