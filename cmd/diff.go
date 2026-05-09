package cmd

import (
	"fmt"
	"strings"
	"unicode"
)

const (
	ansiRed       = "\033[31m"
	ansiGreen     = "\033[32m"
	ansiReset     = "\033[0m"
	ansiRedBg     = "\033[41m"
	ansiGreenBg   = "\033[42m"
	ansiBlackText = "\033[30m"
)

const maxLineDiffCells = 4_000_000

type lineDiffOp struct {
	kind string
	line string
}

type tokenDiffOp struct {
	kind string
	text string
}

func printMarkdownDiff(labelOld, labelNew, oldText, newText string) {
	fmt.Printf("--- %s\n+++ %s\n", labelOld, labelNew)
	for _, group := range groupLineDiff(lineDiff(oldText, newText)) {
		printDiffGroup(group)
	}
}

func groupLineDiff(ops []lineDiffOp) [][]lineDiffOp {
	var groups [][]lineDiffOp
	var current []lineDiffOp
	for _, op := range ops {
		if op.kind == "equal" {
			if len(current) > 0 {
				groups = append(groups, current)
				current = nil
			}
			continue
		}
		current = append(current, op)
	}
	if len(current) > 0 {
		groups = append(groups, current)
	}
	return groups
}

func printDiffGroup(group []lineDiffOp) {
	for len(group) > 0 {
		if len(group) >= 2 && group[0].kind == "delete" && group[1].kind == "insert" {
			printChangedLine(group[0].line, group[1].line)
			group = group[2:]
			continue
		}
		op := group[0]
		switch op.kind {
		case "delete":
			fmt.Printf("%s- %s%s\n", ansiRed, op.line, ansiReset)
		case "insert":
			fmt.Printf("%s+ %s%s\n", ansiGreen, op.line, ansiReset)
		}
		group = group[1:]
	}
}

func printChangedLine(oldLine, newLine string) {
	oldTokens := diffTokens(oldLine)
	newTokens := diffTokens(newLine)
	tokenOps := tokenDiff(oldTokens, newTokens)

	var oldOut strings.Builder
	var newOut strings.Builder
	for _, op := range tokenOps {
		switch op.kind {
		case "equal":
			oldOut.WriteString(op.text)
			newOut.WriteString(op.text)
		case "delete":
			oldOut.WriteString(ansiRedBg + ansiBlackText + op.text + ansiReset + ansiRed)
		case "insert":
			newOut.WriteString(ansiGreenBg + ansiBlackText + op.text + ansiReset + ansiGreen)
		}
	}

	fmt.Printf("%s- %s%s\n", ansiRed, oldOut.String(), ansiReset)
	fmt.Printf("%s+ %s%s\n", ansiGreen, newOut.String(), ansiReset)
}

func lineDiff(oldText, newText string) []lineDiffOp {
	oldLines := strings.Split(oldText, "\n")
	newLines := strings.Split(newText, "\n")
	if len(oldLines)*len(newLines) > maxLineDiffCells {
		return positionalLineDiff(oldLines, newLines)
	}
	table := lcsTable(oldLines, newLines)

	var ops []lineDiffOp
	i, j := 0, 0
	for i < len(oldLines) && j < len(newLines) {
		switch {
		case oldLines[i] == newLines[j]:
			ops = append(ops, lineDiffOp{kind: "equal", line: oldLines[i]})
			i++
			j++
		case table[i+1][j] >= table[i][j+1]:
			ops = append(ops, lineDiffOp{kind: "delete", line: oldLines[i]})
			i++
		default:
			ops = append(ops, lineDiffOp{kind: "insert", line: newLines[j]})
			j++
		}
	}
	for i < len(oldLines) {
		ops = append(ops, lineDiffOp{kind: "delete", line: oldLines[i]})
		i++
	}
	for j < len(newLines) {
		ops = append(ops, lineDiffOp{kind: "insert", line: newLines[j]})
		j++
	}
	return ops
}

func positionalLineDiff(oldLines, newLines []string) []lineDiffOp {
	var ops []lineDiffOp
	common := len(oldLines)
	if len(newLines) < common {
		common = len(newLines)
	}
	for i := 0; i < common; i++ {
		if oldLines[i] == newLines[i] {
			ops = append(ops, lineDiffOp{kind: "equal", line: oldLines[i]})
			continue
		}
		ops = append(ops, lineDiffOp{kind: "delete", line: oldLines[i]})
		ops = append(ops, lineDiffOp{kind: "insert", line: newLines[i]})
	}
	for i := common; i < len(oldLines); i++ {
		ops = append(ops, lineDiffOp{kind: "delete", line: oldLines[i]})
	}
	for i := common; i < len(newLines); i++ {
		ops = append(ops, lineDiffOp{kind: "insert", line: newLines[i]})
	}
	return ops
}

func lcsTable(a, b []string) [][]int {
	table := make([][]int, len(a)+1)
	for i := range table {
		table[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				table[i][j] = table[i+1][j+1] + 1
			} else if table[i+1][j] >= table[i][j+1] {
				table[i][j] = table[i+1][j]
			} else {
				table[i][j] = table[i][j+1]
			}
		}
	}
	return table
}

func diffTokens(line string) []string {
	var tokens []string
	var current strings.Builder
	var currentKind int
	flush := func() {
		if current.Len() == 0 {
			return
		}
		tokens = append(tokens, current.String())
		current.Reset()
	}

	for _, r := range line {
		kind := tokenRuneKind(r)
		if current.Len() > 0 && kind != currentKind {
			flush()
		}
		currentKind = kind
		current.WriteRune(r)
	}
	flush()
	return tokens
}

func tokenRuneKind(r rune) int {
	switch {
	case unicode.IsLetter(r) || unicode.IsDigit(r):
		return 1
	case unicode.IsSpace(r):
		return 2
	default:
		return 3
	}
}

func tokenDiff(oldTokens, newTokens []string) []tokenDiffOp {
	table := tokenLCSTable(oldTokens, newTokens)
	var ops []tokenDiffOp
	i, j := 0, 0
	for i < len(oldTokens) && j < len(newTokens) {
		switch {
		case oldTokens[i] == newTokens[j]:
			ops = appendTokenOp(ops, "equal", oldTokens[i])
			i++
			j++
		case table[i+1][j] >= table[i][j+1]:
			ops = appendTokenOp(ops, "delete", oldTokens[i])
			i++
		default:
			ops = appendTokenOp(ops, "insert", newTokens[j])
			j++
		}
	}
	for i < len(oldTokens) {
		ops = appendTokenOp(ops, "delete", oldTokens[i])
		i++
	}
	for j < len(newTokens) {
		ops = appendTokenOp(ops, "insert", newTokens[j])
		j++
	}
	return ops
}

func tokenLCSTable(a, b []string) [][]int {
	table := make([][]int, len(a)+1)
	for i := range table {
		table[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				table[i][j] = table[i+1][j+1] + 1
			} else if table[i+1][j] >= table[i][j+1] {
				table[i][j] = table[i+1][j]
			} else {
				table[i][j] = table[i][j+1]
			}
		}
	}
	return table
}

func appendTokenOp(ops []tokenDiffOp, kind, text string) []tokenDiffOp {
	if text == "" {
		return ops
	}
	if len(ops) > 0 && ops[len(ops)-1].kind == kind {
		ops[len(ops)-1].text += text
		return ops
	}
	return append(ops, tokenDiffOp{kind: kind, text: text})
}
