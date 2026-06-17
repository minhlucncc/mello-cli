// Command mello is a command-line client for the Mello project-management
// platform: authenticate, manage boards, columns, tickets, comments, and
// attachments, search, and mirror a board to a local working directory for
// offline edits that are synchronized back to the server.
package main

import (
	"os"

	"github.com/minhlucncc/mello-cli/cmd"
)

func main() {
	os.Exit(cmd.Execute(os.Args[1:]))
}
