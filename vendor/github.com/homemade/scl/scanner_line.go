package scl

import (
	"fmt"
	"strings"
)

type lineContent string

func (s lineContent) indent() int {
	return len(s) - len(strings.TrimLeft(string(s), " \t"))
}

type scannerLine struct {
	file     string
	line     int
	column   int
	content  lineContent
	children scannerTree
}

func newLine(fileName string, lineNumber, column int, content string) *scannerLine {
	return &scannerLine{
		file:     fileName,
		line:     lineNumber,
		column:   column,
		content:  lineContent(content),
		children: make(scannerTree, 0),
	}
}

func (l *scannerLine) branch() *scannerLine {
	return newLine(l.file, l.line, l.content.indent(), strings.Trim(string(l.content), " \t"))
}

func (l *scannerLine) String() string {
	return fmt.Sprintf("%s:%d", l.file, l.line)
}
