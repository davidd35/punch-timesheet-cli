package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]
	var err error

	switch cmd {
	case "in":
		err = cmdIn(strings.Join(args, " "))
	case "out":
		err = cmdOut(strings.Join(args, " "))
	case "status":
		err = cmdStatus()
	case "report":
		mode := "week"
		if len(args) > 0 {
			mode = args[0]
		}
		err = cmdReport(mode)
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "punch: unknown command %q\n\n", cmd)
		usage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "punch:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `punch - a plain text time sheet

usage:
  punch in [note]      clock in, optionally with a note
  punch out [note]     clock out
  punch status         show whether you're currently clocked in
  punch report [range] print a summary; range is one of:
                          today, week (default), all

the log lives in ./timesheet.log by default. set TIMESHEET_FILE
to point at a different file, e.g. one per client.
`)
}
