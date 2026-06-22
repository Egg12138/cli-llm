package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/Egg12138/cli-llm/src-go/internal/session/model"
)

type Store struct {
	root string
	name string
	path string
}

func DefaultRoot() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".cli-llm", "sessions")
	}
	return filepath.Join(home, ".cli-llm", "sessions")
}

func Open(root, name string) (Store, error) {
	if err := validateName(name); err != nil {
		return Store{}, err
	}
	if root == "" {
		root = DefaultRoot()
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return Store{}, err
	}
	return Store{
		root: root,
		name: name,
		path: filepath.Join(root, name+".jsonl"),
	}, nil
}

func (s Store) Path() string {
	return s.path
}

func (s *Store) Rename(name string) error {
	if err := validateName(name); err != nil {
		return err
	}
	target := filepath.Join(s.root, name+".jsonl")
	if target == s.path {
		return nil
	}
	if err := os.Rename(s.path, target); err != nil {
		return err
	}
	s.name = name
	s.path = target
	return nil
}

func NameFromTitle(title string) string {
	var builder strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(title)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && builder.Len() > 0 {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	name := strings.Trim(builder.String(), "-")
	if name == "" {
		return "session"
	}
	runes := []rune(name)
	if len(runes) > 48 {
		name = strings.Trim(string(runes[:48]), "-")
	}
	if name == "" {
		return "session"
	}
	return name
}

func (s Store) AvailableName(base string) string {
	name := NameFromTitle(base)
	for i := 1; ; i++ {
		candidate := name
		if i > 1 {
			candidate = fmt.Sprintf("%s-%d", name, i)
		}
		path := filepath.Join(s.root, candidate+".jsonl")
		if path == s.path {
			return candidate
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return candidate
		}
	}
}

func (s Store) Append(entry model.Entry) error {
	file, err := os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	encoded, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(encoded, '\n')); err != nil {
		return err
	}
	return nil
}

func (s Store) Load() ([]model.Entry, error) {
	file, err := os.Open(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []model.Entry{}, nil
		}
		return nil, err
	}
	defer file.Close()

	var entries []model.Entry
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry model.Entry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, fmt.Errorf("parse %s line %d: %w", s.path, lineNumber, err)
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("session name cannot be empty")
	}
	if strings.ContainsAny(name, `/\`) || name == "." || name == ".." || strings.Contains(name, "..") {
		return fmt.Errorf("invalid session name %q", name)
	}
	return nil
}
