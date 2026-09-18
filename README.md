# punch

A command line time sheet. No accounts, no server, no spreadsheet - just a
plain text log you punch in and out of, and a report command that adds it
up for you.

I wanted something to track hours across a couple of freelance clients
without opening a browser tab every time I sit down to work, and without
handing my hours to some SaaS tool that will eventually want a monthly fee.
`punch` writes one line per event to a text file you can read, edit, or
grep yourself.

## Usage

```
$ punch in
clocked in

$ punch status
clocked in since Mon 09:14 (2h03m)

$ punch out "fixed the invoice bug"
clocked out, worked 2h07m

$ punch report today
2026-09-05      2h07m
----------      2h07m

$ punch report week
2026-09-01      4h30m
2026-09-03      6h15m
2026-09-05      2h07m
----------     12h52m
```

`punch report` also takes `all` to summarize the entire log.

## Exporting for invoices

`punch export [range]` prints the sessions in range as CSV instead of a
summary, so you can hand it straight to a spreadsheet:

```
$ punch export week
date,start,end,hours,note
2026-09-01,09:00,13:30,4.50,
2026-09-03,08:45,15:00,6.25,
2026-09-05,09:14,11:21,2.10,fixed the invoice bug
```

Same range names as `report`: `today`, `week` (default), or `all`. A
session that's still open is left with an empty `end`.

## The log file

By default `punch` reads and writes `timesheet.log` in the current
directory. Point `TIMESHEET_FILE` at a different path to keep separate
sheets per client or project:

```
$ TIMESHEET_FILE=~/timesheets/acme.log punch in
```

Each line is tab separated: an RFC 3339 timestamp, `in` or `out`, and an
optional note.

```
2026-09-05T09:14:00-04:00	in
2026-09-05T11:17:00-04:00	out	fixed the invoice bug
```

Because it's just text, fixing a mistake (forgot to clock out, clocked in
twice) is a matter of editing the file in any editor.

## Install

```
go build -o punch .
```

Put the resulting binary somewhere on your `PATH`.

## Status

Early. Clocking in/out, reporting, and CSV export work. See the roadmap
for what's next.
