package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
