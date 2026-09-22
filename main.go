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
	case "log":
		err = cmdLog()
	case "edit":
		err = cmdEdit(args)
	case "remove":
		arg := ""
		if len(args) > 0 {
			arg = args[0]
		}
		err = cmdRemove(arg)
	case "report":
		mode := "week"
		if len(args) > 0 {
			mode = args[0]
		}
		err = cmdReport(mode)
	case "export":
		mode := "week"
		if len(args) > 0 {
			mode = args[0]
		}
		err = cmdExport(mode)
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
  punch log            list every entry with its line number
  punch edit <n> <HH:MM> [note]
                        fix the time (and optionally the note) of entry n
  punch remove <n>     delete entry n
  punch report [range] print a summary; range is one of:
                          today, week (default), all
  punch export [range] print sessions in the range as CSV
                          (date, start, end, hours, note)

the log lives in ./timesheet.log by default. set TIMESHEET_FILE
to point at a different file, e.g. one per client.

use "punch log" to find the line number of a bad entry before
running edit or remove.
`)
}
