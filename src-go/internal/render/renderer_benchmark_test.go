package render

import "testing"

const benchmarkMarkdown = `# Output Benchmark

This paragraph includes **bold text**, ` + "`inline code`" + `, and a short list.

- first item
- second item
- third item

` + "```go" + `
package main

import "fmt"

func main() {
	fmt.Println("hello from cli-llm")
}
` + "```" + `
`

func BenchmarkDefaultRenderMarkdown(b *testing.B) {
	for b.Loop() {
		if _, err := defaultRenderMarkdown(benchmarkMarkdown); err != nil {
			b.Fatal(err)
		}
	}
}
