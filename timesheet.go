package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const defaultLogFile = "timesheet.log"

// Entry is one line of the log: a clock-in or clock-out event.
type Entry struct {
	Time time.Time
	Kind string // "in" or "out"
	Note string
}

func logFilePath() string {
	if v := os.Getenv("TIMESHEET_FILE"); v != "" {
		return v
	}
	return defaultLogFile
}

// readEntries loads the whole log. A missing file just means no entries
// yet, which is the normal state for a brand new time sheet.
func readEntries() ([]Entry, error) {
	data, err := os.ReadFile(logFilePath())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	text := strings.TrimSpace(string(data))
	if text == "" {
		return nil, nil
	}

	lines := strings.Split(text, "\n")
	entries := make([]Entry, 0, len(lines))
	for i, line := range lines {
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) < 2 {
			return nil, fmt.Errorf("%s:%d: malformed line", logFilePath(), i+1)
		}
		t, err := time.Parse(time.RFC3339, fields[0])
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", logFilePath(), i+1, err)
		}
		note := ""
		if len(fields) == 3 {
			note = fields[2]
		}
		entries = append(entries, Entry{Time: t, Kind: fields[1], Note: note})
	}
	return entries, nil
}

func appendEntry(kind, note string) error {
	f, err := os.OpenFile(logFilePath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	line := fmt.Sprintf("%s\t%s\t%s\n", time.Now().Format(time.RFC3339), kind, note)
	_, err = f.WriteString(line)
	return err
}

// writeEntries rewrites the whole log file from scratch. Used by edit and
// remove, which need to replace or drop a line rather than append one.
func writeEntries(entries []Entry) error {
	f, err := os.OpenFile(logFilePath(), os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, e := range entries {
		line := fmt.Sprintf("%s\t%s\t%s\n", e.Time.Format(time.RFC3339), e.Kind, e.Note)
		if _, err := f.WriteString(line); err != nil {
			return err
		}
	}
	return nil
}

// openSession reports the most recent "in" that hasn't been matched by
// an "out" yet, if any. That's the entry that defines "currently clocked in".
func openSession(entries []Entry) *Entry {
	var open *Entry
	for i := range entries {
		e := &entries[i]
		if e.Kind == "in" {
			open = e
		} else if e.Kind == "out" {
			open = nil
		}
	}
	return open
}

func cmdIn(note string) error {
	entries, err := readEntries()
	if err != nil {
		return err
	}
	if open := openSession(entries); open != nil {
		return fmt.Errorf("already clocked in since %s", open.Time.Format("Mon 15:04"))
	}
	if err := appendEntry("in", note); err != nil {
		return err
	}
	fmt.Println("clocked in")
	return nil
}

func cmdOut(note string) error {
	entries, err := readEntries()
	if err != nil {
		return err
	}
	open := openSession(entries)
	if open == nil {
		return errors.New("not clocked in")
	}
	if err := appendEntry("out", note); err != nil {
		return err
	}
	fmt.Printf("clocked out, worked %s\n", formatDuration(time.Since(open.Time)))
	return nil
}

func cmdStatus() error {
	entries, err := readEntries()
	if err != nil {
		return err
	}
	open := openSession(entries)
	if open == nil {
		fmt.Println("not clocked in")
		return nil
	}
	fmt.Printf("clocked in since %s (%s)\n", open.Time.Format("Mon 15:04"), formatDuration(time.Since(open.Time)))
	return nil
}

// cmdLog prints every raw entry with a 1-based line number, which is what
// edit and remove take as their target. It's the "look before you fix"
// step for correcting a bad punch.
func cmdLog() error {
	entries, err := readEntries()
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Println("no entries")
		return nil
	}
	for i, e := range entries {
		note := ""
		if e.Note != "" {
			note = "  " + e.Note
		}
		fmt.Printf("%3d  %s  %-3s%s\n", i+1, e.Time.Format("2006-01-02 15:04"), e.Kind, note)
	}
	return nil
}

func cmdEdit(args []string) error {
	if len(args) < 2 {
		return errors.New("usage: punch edit <line number> <HH:MM> [note] (see punch log)")
	}
	n, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("%q is not a line number (see punch log)", args[0])
	}

	entries, err := readEntries()
	if err != nil {
		return err
	}
	if n < 1 || n > len(entries) {
		return fmt.Errorf("no entry %d (see punch log)", n)
	}

	e := &entries[n-1]
	newTime, err := parseTimeOfDay(args[1], e.Time)
	if err != nil {
		return err
	}
	e.Time = newTime
	if len(args) > 2 {
		e.Note = strings.Join(args[2:], " ")
	}

	if err := writeEntries(entries); err != nil {
		return err
	}
	fmt.Printf("updated entry %d: %s %s\n", n, e.Time.Format("2006-01-02 15:04"), e.Kind)
	return nil
}

// parseTimeOfDay parses "HH:MM" as a time on the same day as ref, in ref's
// location. That's what you want when fixing a fat-fingered clock time
// without having to retype the whole date.
func parseTimeOfDay(s string, ref time.Time) (time.Time, error) {
	t, err := time.ParseInLocation("15:04", s, ref.Location())
	if err != nil {
		return time.Time{}, fmt.Errorf("%q is not a time in HH:MM form", s)
	}
	return time.Date(ref.Year(), ref.Month(), ref.Day(), t.Hour(), t.Minute(), 0, 0, ref.Location()), nil
}

func cmdRemove(arg string) error {
	if arg == "" {
		return errors.New("usage: punch remove <line number> (see punch log)")
	}
	n, err := strconv.Atoi(arg)
	if err != nil {
		return fmt.Errorf("%q is not a line number (see punch log)", arg)
	}

	entries, err := readEntries()
	if err != nil {
		return err
	}
	if n < 1 || n > len(entries) {
		return fmt.Errorf("no entry %d (see punch log)", n)
	}

	removed := entries[n-1]
	entries = append(entries[:n-1], entries[n:]...)
	if err := writeEntries(entries); err != nil {
		return err
	}
	fmt.Printf("removed entry %d: %s %s\n", n, removed.Time.Format("2006-01-02 15:04"), removed.Kind)
	return nil
}

// Session is a completed or in-progress span of work, keyed by the day
// it started on. A session that started before midnight and ended after
// is credited to the day it started, which matches how most people would
// describe "the shift I worked last night".
type Session struct {
	Day      string
	Start    time.Time
	End      time.Time
	Duration time.Duration
	Note     string
	Ongoing  bool
}

func buildSessions(entries []Entry) []Session {
	var sessions []Session
	var start *time.Time

	for i := range entries {
		e := entries[i]
		switch e.Kind {
		case "in":
			t := e.Time
			start = &t
		case "out":
			if start != nil {
				sessions = append(sessions, Session{
					Day:      start.Format("2006-01-02"),
					Start:    *start,
					End:      e.Time,
					Duration: e.Time.Sub(*start),
					Note:     e.Note,
				})
				start = nil
			}
		}
	}

	if start != nil {
		sessions = append(sessions, Session{
			Day:      start.Format("2006-01-02"),
			Start:    *start,
			Duration: time.Since(*start),
			Ongoing:  true,
		})
	}

	return sessions
}

// rangeSince turns a report/export range name into the cutoff time a
// session's day must fall on or after to be included.
func rangeSince(mode string) (time.Time, error) {
	switch mode {
	case "today":
		now := time.Now()
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()), nil
	case "week":
		return time.Now().AddDate(0, 0, -7), nil
	case "all":
		return time.Time{}, nil
	default:
		return time.Time{}, fmt.Errorf("unknown range %q (want today, week, or all)", mode)
	}
}

// sessionsInRange filters completed and ongoing sessions down to those
// whose day is on or after since.
func sessionsInRange(entries []Entry, since time.Time) []Session {
	var out []Session
	for _, s := range buildSessions(entries) {
		day, err := time.ParseInLocation("2006-01-02", s.Day, time.Local)
		if err != nil || day.Before(since) {
			continue
		}
		out = append(out, s)
	}
	return out
}

func cmdReport(mode string) error {
	entries, err := readEntries()
	if err != nil {
		return err
	}

	since, err := rangeSince(mode)
	if err != nil {
		return err
	}

	totals := map[string]time.Duration{}
	ongoing := map[string]bool{}
	for _, s := range sessionsInRange(entries, since) {
		totals[s.Day] += s.Duration
		if s.Ongoing {
			ongoing[s.Day] = true
		}
	}

	if len(totals) == 0 {
		fmt.Println("no time logged in that range")
		return nil
	}

	days := make([]string, 0, len(totals))
	for d := range totals {
		days = append(days, d)
	}
	sort.Strings(days)

	var grand time.Duration
	for _, d := range days {
		mark := ""
		if ongoing[d] {
			mark = " (in progress)"
		}
		fmt.Printf("%s  %8s%s\n", d, formatDuration(totals[d]), mark)
		grand += totals[d]
	}
	fmt.Printf("%s  %8s\n", strings.Repeat("-", 10), formatDuration(grand))

	return nil
}

// cmdExport writes one CSV row per session in the given range to stdout,
// with hours as a decimal so the output can be dropped straight into an
// invoice spreadsheet.
func cmdExport(mode string) error {
	entries, err := readEntries()
	if err != nil {
		return err
	}

	since, err := rangeSince(mode)
	if err != nil {
		return err
	}

	sessions := sessionsInRange(entries, since)
	if len(sessions) == 0 {
		return errors.New("no time logged in that range")
	}

	return writeCSV(os.Stdout, sessions)
}

func writeCSV(w io.Writer, sessions []Session) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"date", "start", "end", "hours", "note"}); err != nil {
		return err
	}

	for _, s := range sessions {
		end := ""
		if !s.Ongoing {
			end = s.End.Format("15:04")
		}
		row := []string{
			s.Day,
			s.Start.Format("15:04"),
			end,
			fmt.Sprintf("%.2f", s.Duration.Hours()),
			s.Note,
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}

	cw.Flush()
	return cw.Error()
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	h := d / time.Hour
	m := (d % time.Hour) / time.Minute
	return fmt.Sprintf("%dh%02dm", h, m)
}
