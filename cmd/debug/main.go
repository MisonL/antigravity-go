package main

import (
	"fmt"
	"os"

	_ "github.com/mison/antigravity-go/internal/agent"
	_ "github.com/mison/antigravity-go/internal/core"
	_ "github.com/mison/antigravity-go/internal/server"
	_ "github.com/mison/antigravity-go/internal/tui"
)

func main() {
	fmt.Println("Debug: Binary with Agent import started")
	os.Stdout.Sync()
}
