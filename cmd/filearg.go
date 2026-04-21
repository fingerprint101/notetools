package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// parseFileArg parses "path/to/file.md:10-50" or "path/to/file.md:5" into (path, startLine, endLine, err).
// If no range is given, start=0 and end=0 meaning "whole file".
// Lines are 1-indexed.
func parseFileArg(arg string) (path string, start int, end int, err error) {
	idx := strings.LastIndex(arg, ":")
	if idx > 0 {
		suffix := arg[idx+1:]
		parts := strings.SplitN(suffix, "-", 2)
		if len(parts) == 2 {
			s, errS := strconv.Atoi(parts[0])
			e, errE := strconv.Atoi(parts[1])
			if errS == nil && errE == nil && s > 0 && e > 0 {
				if s > e {
					return "", 0, 0, fmt.Errorf("invalid range: start (%d) is greater than end (%d)", s, e)
				}
				return arg[:idx], s, e, nil
			}
		} else if len(parts) == 1 {
			s, errS := strconv.Atoi(parts[0])
			if errS == nil && s > 0 {
				return arg[:idx], s, s, nil
			}
		}
	}
	return arg, 0, 0, nil
}

// readLines reads a file and returns the content between start and end lines (1-indexed, inclusive).
// If start=0 and end=0, returns the entire file content.
func readLines(path string, start, end int) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("file not found: %s", path)
	}

	lines := strings.Split(string(data), "\n")

	if start == 0 && end == 0 {
		return string(data), nil
	}

	if start > len(lines) {
		return "", fmt.Errorf("start line %d exceeds file length (%d lines)", start, len(lines))
	}
	if end > len(lines) {
		end = len(lines)
	}

	selected := lines[start-1 : end]
	return strings.Join(selected, "\n"), nil
}

// replaceLines replaces lines start through end (1-indexed, inclusive) in the file with newContent.
// If start=0 and end=0, replaces the entire file content.
func replaceLines(path string, start, end int, newContent string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("file not found: %s", path)
	}

	if start == 0 && end == 0 {
		return os.WriteFile(path, []byte(newContent), 0o644)
	}

	lines := strings.Split(string(data), "\n")

	if start > len(lines) {
		return fmt.Errorf("start line %d exceeds file length (%d lines)", start, len(lines))
	}
	if end > len(lines) {
		end = len(lines)
	}

	newLines := strings.Split(newContent, "\n")
	result := make([]string, 0, len(lines)-(end-start+1)+len(newLines))
	result = append(result, lines[:start-1]...)
	result = append(result, newLines...)
	result = append(result, lines[end:]...)

	return os.WriteFile(path, []byte(strings.Join(result, "\n")), 0o644)
}
