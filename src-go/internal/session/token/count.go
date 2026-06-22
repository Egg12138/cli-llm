package token

import tiktoken "github.com/pkoukk/tiktoken-go"

const (
	DefaultCompressionThreshold = 200000
	DefaultKeepRecentTokens     = 20000
)

func CountText(text, modelName string) (int, error) {
	encoding, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		return 0, err
	}
	return len(encoding.Encode(text, nil, nil)), nil
}

func ShouldCompress(tokens, threshold int) bool {
	if threshold <= 0 {
		threshold = DefaultCompressionThreshold
	}
	return tokens >= threshold
}
