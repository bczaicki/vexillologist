package main

import (
	"os"

	"github.com/phillips-connect/vexillologist/internal/cli"
	"github.com/phillips-connect/vexillologist/internal/tui"
)

func main() {
	os.Exit(cli.Execute(tui.Run))
}
