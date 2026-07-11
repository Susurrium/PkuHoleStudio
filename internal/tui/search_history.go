package tui

import (
	"bufio"
	"os"
	"strings"

	"github.com/Susurrium/PkuHoleStudio/internal/config"
)

const searchHistoryMaxEntries = 200

func loadSearchHistory() []string {
	path, err := config.SearchHistoryPath()
	if err != nil {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var entries []string
	seen := map[string]bool{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || seen[line] {
			continue
		}
		seen[line] = true
		entries = append(entries, line)
	}
	if len(entries) > searchHistoryMaxEntries {
		entries = entries[len(entries)-searchHistoryMaxEntries:]
	}
	return entries
}

func saveSearchHistory(entries []string) {
	path, err := config.SearchHistoryPath()
	if err != nil {
		return
	}
	if len(entries) > searchHistoryMaxEntries {
		entries = entries[len(entries)-searchHistoryMaxEntries:]
	}
	var b strings.Builder
	for _, e := range entries {
		b.WriteString(e)
		b.WriteByte('\n')
	}
	_ = os.WriteFile(path, []byte(b.String()), 0644)
}

func appendSearchHistory(entry string) []string {
	history := loadSearchHistory()
	var filtered []string
	for _, e := range history {
		if e != entry {
			filtered = append(filtered, e)
		}
	}
	filtered = append(filtered, entry)
	saveSearchHistory(filtered)
	return filtered
}
