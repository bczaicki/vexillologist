package main

import (
	"os"

	"github.com/bczaicki/vexillologist/internal/cli"
	"github.com/bczaicki/vexillologist/internal/tui"
)

func main() {
	os.Exit(cli.Execute(tui.Run))
}
