package token

import "testing"

func TestTokenCountTextUsesFallbackEncoding(t *testing.T) {
	t.Parallel()

	count, err := CountText("hello world", "unknown-model")
	if err != nil {
		t.Fatalf("CountText returned error: %v", err)
	}
	if count <= 0 {
		t.Fatalf("expected positive token count, got %d", count)
	}
}

func TestTokenShouldCompress(t *testing.T) {
	t.Parallel()

	if ShouldCompress(99, 100) {
		t.Fatalf("should not compress below threshold")
	}
	if !ShouldCompress(100, 100) {
		t.Fatalf("should compress at threshold")
	}
}
