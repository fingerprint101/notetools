package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// parseFileArg parses "path/to/file.md:10-50" into (path, startLine, endLine, err).
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
