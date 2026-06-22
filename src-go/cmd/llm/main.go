package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/Egg12138/cli-llm/src-go/internal/cli"
)

func main() {
	code, err := cli.Execute(os.Args[1:], exec.LookPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(code)
}
