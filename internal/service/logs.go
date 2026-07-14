package service

import (
	"bufio"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type LogLine struct {
	Module string `json:"module"`
	Line   string `json:"line"`
}

type LogService struct{ dataDir string }

func NewLogService(dataDir string) *LogService { return &LogService{dataDir: dataDir} }

// Stream follows new redacted log lines. Callers should load the current tail
// with List before subscribing; the stream starts at the current end of file.
func (s *LogService) Stream(ctx context.Context, module, query string) (<-chan LogLine, error) {
	modules, err := logModules(module)
	if err != nil {
		return nil, err
	}
	offsets := make(map[string]int64, len(modules))
	for _, current := range modules {
		if info, statErr := os.Stat(filepath.Join(s.dataDir, current+".log")); statErr == nil {
			offsets[current] = info.Size()
		}
	}
	result := make(chan LogLine, 64)
	go func() {
		defer close(result)
		ticker := time.NewTicker(750 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				for _, current := range modules {
					lines, next := s.readNewLines(current, offsets[current])
					offsets[current] = next
					for _, line := range lines {
						if query != "" && !strings.Contains(strings.ToLower(line), strings.ToLower(query)) {
							continue
						}
						select {
						case result <- LogLine{Module: current, Line: line}:
						case <-ctx.Done():
							return
						}
					}
				}
			}
		}
	}()
	return result, nil
}

func (s *LogService) readNewLines(module string, offset int64) ([]string, int64) {
	path := filepath.Join(s.dataDir, module+".log")
	file, err := os.Open(path)
	if err != nil {
		return nil, offset
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, offset
	}
	if info.Size() < offset {
		offset = 0
	}
	if _, err := file.Seek(offset, 0); err != nil {
		return nil, offset
	}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1<<20)
	lines := make([]string, 0)
	for scanner.Scan() {
		lines = append(lines, redactLogLine(scanner.Text(), s.dataDir))
	}
	next, err := file.Seek(0, 1)
	if err != nil {
		return lines, offset
	}
	return lines, next
}

func (s *LogService) List(ctx context.Context, module, query string, limit int) ([]LogLine, error) {
	if limit <= 0 || limit > 5_000 {
		limit = 500
	}
	modules, err := logModules(module)
	if err != nil {
		return nil, err
	}
	result := make([]LogLine, 0)
	for _, current := range modules {
		file, err := os.Open(filepath.Join(s.dataDir, current+".log"))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 64*1024), 1<<20)
		for scanner.Scan() {
			if err := ctx.Err(); err != nil {
				_ = file.Close()
				return nil, err
			}
			line := redactLogLine(scanner.Text(), s.dataDir)
			if query == "" || strings.Contains(strings.ToLower(line), strings.ToLower(query)) {
				result = append(result, LogLine{Module: current, Line: line})
				if len(result) > limit {
					result = result[len(result)-limit:]
				}
			}
		}
		scanErr := scanner.Err()
		_ = file.Close()
		if scanErr != nil {
			return nil, scanErr
		}
	}
	return result, nil
}

func (s *LogService) Clear(ctx context.Context, module string) error {
	modules, err := logModules(module)
	if err != nil {
		return err
	}
	for _, current := range modules {
		if err := ctx.Err(); err != nil {
			return err
		}
		path := filepath.Join(s.dataDir, current+".log")
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			return err
		}
	}
	return nil
}

func logModules(module string) ([]string, error) {
	switch strings.ToLower(strings.TrimSpace(module)) {
	case "", "all":
		return []string{"crawler", "tui"}, nil
	case "crawler", "tui":
		return []string{strings.ToLower(strings.TrimSpace(module))}, nil
	default:
		return nil, errors.New("unsupported log module")
	}
}

func redactLogLine(line, dataDir string) string {
	line = strings.ReplaceAll(line, dataDir, "<data-dir>")
	for _, marker := range []string{"Authorization: Bearer ", "api_key=", "token="} {
		if index := strings.Index(strings.ToLower(line), strings.ToLower(marker)); index >= 0 {
			line = line[:index] + marker + "<redacted>"
		}
	}
	return line
}
