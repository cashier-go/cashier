package scl

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type scannerTree []*scannerLine

type scanner struct {
	file   string
	reader io.Reader
	lines  scannerTree
}

func newScanner(reader io.Reader, filename ...string) *scanner {

	file := "<no file>"

	if len(filename) > 0 {
		file = filename[0]
	}

	s := scanner{
		file:   file,
		reader: reader,
		lines:  make(scannerTree, 0),
	}

	return &s
}

func (s *scanner) scan() (lines scannerTree, err error) {

	// Split to lines
	scanner := bufio.NewScanner(s.reader)
	scanner.Split(bufio.ScanLines)

	lineNumber := 0
	rawLines := make(scannerTree, 0)

	heredoc := ""
	heredocContent := ""
	heredocLine := 0

	for scanner.Scan() {
		lineNumber++

		if heredoc != "" {
			heredocContent += "\n" + scanner.Text()

			if strings.TrimSpace(scanner.Text()) == heredoc {
				// HCL requires heredocs to be terminated with a newline
				rawLines = append(rawLines, newLine(s.file, lineNumber, 0, heredocContent+"\n"))
				heredoc = ""
				heredocContent = ""
			}

			continue
		}

		text := strings.TrimRight(scanner.Text(), " \t{}")

		if text == "" {
			continue
		}

		if matches := heredocMatcher.FindAllStringSubmatch(text, -1); matches != nil {
			heredoc = matches[0][1]
			heredocContent = text
			heredocLine = lineNumber
			continue
		}

		rawLines = append(rawLines, newLine(s.file, lineNumber, 0, text))
	}

	if heredoc != "" {
		return lines, fmt.Errorf("Heredoc '%s' (started line %d) not terminated", heredoc, heredocLine)
	}

	// Make sure the first line has no indent
	if len(rawLines) > 0 {
		index := 0
		s.indentLines(&index, rawLines, &lines, rawLines[0].content.indent())
	}

	return
}

func (s *scanner) indentLines(index *int, input scannerTree, output *scannerTree, indent int) {

	// Ends when there are no more lines
	if *index >= len(input) {
		return
	}

	var lineToAdd *scannerLine

	for ; *index < len(input); *index++ {

		lineIndent := input[*index].content.indent()

		if lineIndent == indent {
			lineToAdd = input[*index].branch()
			*output = append(*output, lineToAdd)

		} else if lineIndent > indent {
			s.indentLines(index, input, &lineToAdd.children, lineIndent)

		} else if lineIndent < indent {
			*index--
			return
		}

	}

	return
}
