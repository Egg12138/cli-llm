// Resume and manage chat sessions with history.
package main

import (
	"os"

	sessioncli "github.com/Egg12138/cli-llm/src-go/internal/session/cli"
)

func main() {
	os.Exit(sessioncli.Main(os.Args[1:]))
}
