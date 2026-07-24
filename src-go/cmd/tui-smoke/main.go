package main

import (
	"fmt"
	"os"
	"time"

	"github.com/Egg12138/cli-llm/src-go/internal/session/graph"
	"github.com/Egg12138/cli-llm/src-go/internal/session/model"
	"github.com/Egg12138/cli-llm/src-go/internal/session/tui"
)

func must(entry model.Entry, err error) model.Entry {
	if err != nil {
		panic(err)
	}
	return entry
}

func main() {
	now := time.Date(2026, 6, 23, 10, 0, 0, 0, time.UTC)
	root := must(model.NewMessage("", "user", "common root prompt", now))
	mainReply := must(model.NewMessage(root.ID, "assistant", "main-only reply\n\n```go\nfmt.Println(\"main\")\n```", now.Add(time.Second)))
	exploreReply := must(model.NewMessage(root.ID, "assistant", "explore-only reply\n\n- branch panel should hide main-only reply\n- y should copy raw text", now.Add(2*time.Second)))

	state := graph.NewState([]model.Entry{root, mainReply, exploreReply})
	state.Branches = map[string]graph.Branch{
		"main":    {Name: "main", HeadID: mainReply.ID},
		"explore": {Name: "explore", HeadID: exploreReply.ID, Parent: "main"},
	}
	state.CurrentBranch = "main"
	state.HeadID = mainReply.ID

	if err := tui.NewOverlay().Open(state, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
