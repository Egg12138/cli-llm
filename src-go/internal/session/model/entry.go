package model

import (
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	EntryTypeMessage       = "message"
	EntryTypeCheckpoint    = "custom:checkpoint"
	EntryTypeSessionInfo   = "custom:session_info"
	EntryTypeCompaction    = "compaction"
	EntryTypeBranchSummary = "branch_summary"
)

type Entry struct {
	ID        string          `json:"id"`
	ParentID  string          `json:"parentId,omitempty"`
	Type      string          `json:"type"`
	CreatedAt time.Time       `json:"createdAt"`
	Data      json.RawMessage `json:"data"`
}

type MessageData struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type CheckpointData struct {
	Name     string `json:"name,omitempty"`
	ReturnTo string `json:"returnTo"`
}

type SessionInfo struct {
	Title   string    `json:"title"`
	Created time.Time `json:"created"`
	Model   string    `json:"model"`
}

type CompactionData struct {
	Summary          string `json:"summary"`
	FirstKeptEntryID string `json:"firstKeptEntryId"`
	TokensBefore     int    `json:"tokensBefore"`
}

type BranchSummaryData struct {
	Summary string `json:"summary"`
	FromID  string `json:"fromId"`
}

func NewMessage(parentID, role, content string, now time.Time) (Entry, error) {
	return newEntry(parentID, EntryTypeMessage, MessageData{
		Role:    role,
		Content: content,
	}, now)
}

func NewCheckpoint(parentID, name, returnTo string, now time.Time) (Entry, error) {
	return newEntry(parentID, EntryTypeCheckpoint, CheckpointData{
		Name:     name,
		ReturnTo: returnTo,
	}, now)
}

func NewSessionInfo(parentID string, info SessionInfo, now time.Time) (Entry, error) {
	return newEntry(parentID, EntryTypeSessionInfo, info, now)
}

func NewCompaction(parentID string, data CompactionData, now time.Time) (Entry, error) {
	return newEntry(parentID, EntryTypeCompaction, data, now)
}

func NewBranchSummary(parentID string, data BranchSummaryData, now time.Time) (Entry, error) {
	return newEntry(parentID, EntryTypeBranchSummary, data, now)
}

func newEntry(parentID, entryType string, data any, now time.Time) (Entry, error) {
	encoded, err := json.Marshal(data)
	if err != nil {
		return Entry{}, err
	}
	entry := Entry{
		ParentID:  parentID,
		Type:      entryType,
		CreatedAt: now.UTC(),
		Data:      encoded,
	}
	id, err := hashEntry(entry)
	if err != nil {
		return Entry{}, err
	}
	entry.ID = id
	return entry, nil
}

func (e Entry) MessageData() (MessageData, error) {
	var data MessageData
	err := json.Unmarshal(e.Data, &data)
	return data, err
}

func (e Entry) CheckpointData() (CheckpointData, error) {
	var data CheckpointData
	err := json.Unmarshal(e.Data, &data)
	return data, err
}

func (e Entry) SessionInfoData() (SessionInfo, error) {
	var data SessionInfo
	err := json.Unmarshal(e.Data, &data)
	return data, err
}

func (e Entry) CompactionData() (CompactionData, error) {
	var data CompactionData
	err := json.Unmarshal(e.Data, &data)
	return data, err
}

func (e Entry) BranchSummaryData() (BranchSummaryData, error) {
	var data BranchSummaryData
	err := json.Unmarshal(e.Data, &data)
	return data, err
}

func (e Entry) Preview() (string, error) {
	if e.Type != EntryTypeMessage {
		return "", nil
	}
	data, err := e.MessageData()
	if err != nil {
		return "", err
	}
	content := strings.TrimSpace(data.Content)
	if utf8.RuneCountInString(content) <= 16 {
		return content, nil
	}
	runes := []rune(content)
	return string(runes[:16]), nil
}
