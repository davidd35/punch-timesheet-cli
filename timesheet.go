package main

import (
	"errors"
	"fmt"
	"os"
	"sort"
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

// Session is a completed or in-progress span of work, keyed by the day
// it started on. A session that started before midnight and ended after
// is credited to the day it started, which matches how most people would
// describe "the shift I worked last night".
type Session struct {
	Day      string
	Duration time.Duration
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
					Duration: e.Time.Sub(*start),
				})
				start = nil
			}
		}
	}

	if start != nil {
		sessions = append(sessions, Session{
			Day:      start.Format("2006-01-02"),
			Duration: time.Since(*start),
			Ongoing:  true,
		})
	}

	return sessions
}

func cmdReport(mode string) error {
	entries, err := readEntries()
	if err != nil {
		return err
	}

	var since time.Time
	switch mode {
	case "today":
		now := time.Now()
		since = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case "week":
		since = time.Now().AddDate(0, 0, -7)
	case "all":
		since = time.Time{}
	default:
		return fmt.Errorf("unknown report range %q (want today, week, or all)", mode)
	}

	totals := map[string]time.Duration{}
	ongoing := map[string]bool{}
	for _, s := range buildSessions(entries) {
		day, err := time.ParseInLocation("2006-01-02", s.Day, time.Local)
		if err != nil || day.Before(since) {
			continue
		}
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

func formatDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	h := d / time.Hour
	m := (d % time.Hour) / time.Minute
	return fmt.Sprintf("%dh%02dm", h, m)
}
