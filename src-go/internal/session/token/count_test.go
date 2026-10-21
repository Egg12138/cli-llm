package token

import (
	"strings"
	"testing"
)

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

func TestNeedsCompressionSkipsTextShorterThanThreshold(t *testing.T) {
	t.Parallel()

	for _, text := range []string{"", "short", strings.Repeat("x", 99)} {
		tokens, needed, err := NeedsCompression(text, "bench-model", 100)
		if err != nil {
			t.Fatalf("NeedsCompression(%d bytes) returned error: %v", len(text), err)
		}
		if needed || tokens != 0 {
			t.Fatalf("expected %d-byte text to be skipped, got tokens=%d needed=%t", len(text), tokens, needed)
		}
	}
}

func TestNeedsCompressionCountsTextThatCanReachThreshold(t *testing.T) {
	t.Parallel()

	text := strings.Repeat("hello world ", 20) // 240 bytes, ~60 tokens

	tokens, needed, err := NeedsCompression(text, "bench-model", 100)
	if err != nil {
		t.Fatalf("NeedsCompression returned error: %v", err)
	}
	if needed {
		t.Fatalf("expected ~60 tokens to stay below a 100-token threshold")
	}
	if tokens == 0 || tokens >= 100 {
		t.Fatalf("expected an exact count below the threshold, got %d", tokens)
	}

	tokens, needed, err = NeedsCompression(text, "bench-model", 3)
	if err != nil {
		t.Fatalf("NeedsCompression returned error: %v", err)
	}
	if !needed || tokens < 3 {
		t.Fatalf("expected compression at a 3-token threshold, got tokens=%d needed=%t", tokens, needed)
	}
}

func TestNeedsCompressionTreatsZeroThresholdAsDefault(t *testing.T) {
	t.Parallel()

	tokens, needed, err := NeedsCompression("short", "bench-model", 0)
	if err != nil {
		t.Fatalf("NeedsCompression returned error: %v", err)
	}
	if needed || tokens != 0 {
		t.Fatalf("expected short text to be skipped, got tokens=%d needed=%t", tokens, needed)
	}
}

func BenchmarkCountText(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := CountText("hello world", "bench-model"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNeedsCompressionShortHistory(b *testing.B) {
	text := strings.Repeat("user: short context\n", 40)
	b.ReportAllocs()
	for b.Loop() {
		if _, needed, err := NeedsCompression(text, "bench-model", DefaultCompressionThreshold); err != nil {
			b.Fatal(err)
		} else if needed {
			b.Fatal("unexpected compression for short history")
		}
	}
}
