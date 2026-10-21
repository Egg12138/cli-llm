package token

import (
	"sync"

	tiktoken "github.com/pkoukk/tiktoken-go"
)

const (
	DefaultCompressionThreshold = 200000
	DefaultKeepRecentTokens     = 20000
)

var (
	encoderOnce sync.Once
	encoder     *tiktoken.Tiktoken
	encoderErr  error
)

// getEncoder builds the cl100k encoder once per process. tiktoken-go caches the
// merged ranks but rebuilds the CoreBPE (regexp compilation and a ~100k-entry
// decoder map) on every GetEncoding call, which costs tens of milliseconds.
func getEncoder() (*tiktoken.Tiktoken, error) {
	encoderOnce.Do(func() {
		encoder, encoderErr = tiktoken.GetEncoding("cl100k_base")
	})
	return encoder, encoderErr
}

func CountText(text, modelName string) (int, error) {
	encoding, err := getEncoder()
	if err != nil {
		return 0, err
	}
	return len(encoding.Encode(text, nil, nil)), nil
}

// NeedsCompression reports whether text reaches the compression threshold and
// returns the exact token count when it does. Every BPE token encodes at least
// one byte, so tokens can never exceed len(text); histories shorter than the
// token threshold are skipped without building the tokenizer. That keeps the
// compaction check off the startup path for ordinary sessions.
func NeedsCompression(text, modelName string, threshold int) (int, bool, error) {
	if threshold <= 0 {
		threshold = DefaultCompressionThreshold
	}
	if len(text) < threshold {
		return 0, false, nil
	}
	tokens, err := CountText(text, modelName)
	if err != nil {
		return 0, false, err
	}
	return tokens, ShouldCompress(tokens, threshold), nil
}

func ShouldCompress(tokens, threshold int) bool {
	if threshold <= 0 {
		threshold = DefaultCompressionThreshold
	}
	return tokens >= threshold
}
