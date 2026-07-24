package acceptance

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"
)

type mockMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type mockRequest struct {
	Stream   bool          `json:"stream"`
	Messages []mockMessage `json:"messages"`
}

type mockOpenAI struct {
	server   *httptest.Server
	mu       sync.Mutex
	requests []mockRequest
}

func newMockOpenAI() *mockOpenAI {
	mock := &mockOpenAI{}
	mock.server = httptest.NewServer(http.HandlerFunc(mock.serveHTTP))
	return mock
}

func (m *mockOpenAI) Close() {
	m.server.Close()
}

func (m *mockOpenAI) URL() string {
	return m.server.URL + "/v1"
}

func (m *mockOpenAI) Requests() []mockRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	requests := make([]mockRequest, len(m.requests))
	copy(requests, m.requests)
	return requests
}

func (m *mockOpenAI) StreamingUsers() []string {
	requests := m.Requests()
	users := make([]string, 0, len(requests))
	for _, request := range requests {
		if request.Stream {
			users = append(users, lastUserMessage(request.Messages))
		}
	}
	return users
}

func (m *mockOpenAI) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
		http.NotFound(w, r)
		return
	}
	var request mockRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	m.mu.Lock()
	m.requests = append(m.requests, request)
	m.mu.Unlock()

	if !request.Stream {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"id":"title","object":"chat.completion","created":1,"model":"mock-model","choices":[{"index":0,"message":{"role":"assistant","content":"PTY Session"},"finish_reason":"stop"}]}`)
		return
	}

	user := lastUserMessage(request.Messages)
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	if user == "cancel me" {
		<-r.Context().Done()
		return
	}

	time.Sleep(180 * time.Millisecond)
	for _, chunk := range responseChunks(user) {
		payload, _ := json.Marshal(map[string]any{
			"id":      "chunk",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "mock-model",
			"choices": []map[string]any{{
				"index":         0,
				"delta":         map[string]string{"content": chunk},
				"finish_reason": nil,
			}},
		})
		_, _ = fmt.Fprintf(w, "data: %s\n\n", payload)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		time.Sleep(5 * time.Millisecond)
	}
	_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func responseChunks(user string) []string {
	switch user {
	case "你好 世界":
		return []string{"CJK_", "ACK\n"}
	case "第一行\nsecond line":
		chunks := make([]string, 0, 38)
		for i := 0; i < 36; i++ {
			chunks = append(chunks, fmt.Sprintf("LONG-LINE-%02d unique response marker\n", i))
		}
		chunks = append(chunks, strings.Repeat("中文宽字符", 18)+"\n", "MULTILINE_ACK\n")
		return chunks
	case "resize 保留":
		return []string{"RESIZE_ACK\n"}
	default:
		return []string{"ACK\n"}
	}
}

func lastUserMessage(messages []mockMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return messages[i].Content
		}
	}
	return ""
}
