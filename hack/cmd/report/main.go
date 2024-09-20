// Copyright 2024 Registry Operator contributors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

const (
	Failed = ":x: Failed"
	Passed = ":white_check_mark: Passed"
)

// Table structure to represent headers and rows
type Table struct {
	Headers   []string
	Separator []string
	Rows      [][]string
}

// NewTable initializes a new table
func NewTable(headers []string) *Table {
	return &Table{
		Headers:   headers,
		Separator: make([]string, len(headers)),
		Rows:      [][]string{},
	}
}

// Add adds a row to the table
func (t *Table) Add(row []string) {
	if len(row) != len(t.Headers) {
		panic("row length does not match header length")
	}
	t.Rows = append(t.Rows, row)
}

// Report structure to represent the test report
type Report struct {
	Time      string
	Timestamp string
	Table     *Table
	OK        bool
	Failures  int
	Passes    int
}

// NewReport creates a new report
func NewReport(time, timestamp string, table *Table) *Report {
	failures := 0
	passes := 0

	for _, row := range table.Rows {
		if contains(row, Failed) {
			failures++
		} else if contains(row, Passed) {
			passes++
		}
	}

	return &Report{
		Time:      time,
		Timestamp: timestamp,
		Table:     table,
		OK:        failures == 0,
		Failures:  failures,
		Passes:    passes,
	}
}

// Print outputs the report to the provided writer as markdown
func (r *Report) Print(w io.Writer) {
	fmt.Fprintf(w, "## E2E report %s\n", ifElse(r.OK, Passed, Failed))
	fmt.Fprintf(w, "Started at `%s` took `%s`\n\n", r.Timestamp, r.Time)
	fmt.Fprintf(w, "![](https://img.shields.io/badge/tests-%d_passed%%2C_%d_failed-%s)\n\n",
		r.Passes, r.Failures, ifElse(r.OK, "green", "red"))

	// Print table headers and separator
	fmt.Fprintln(w, strings.Join(r.Table.Headers, "|"))
	fmt.Fprintln(w, strings.Join(r.Table.Separator, "|"))

	// Sort and print table rows by status
	sort.Slice(r.Table.Rows, func(i, j int) bool {
		return r.Table.Rows[i][len(r.Table.Rows[i])-1] > r.Table.Rows[j][len(r.Table.Rows[j])-1]
	})
	for _, row := range r.Table.Rows {
		fmt.Fprintln(w, strings.Join(row, "|"))
	}
}

// contains checks if a slice contains a string
func contains(slice []string, value string) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

// ifElse returns trueVal if condition is true, else falseVal
func ifElse(condition bool, trueVal, falseVal string) string {
	if condition {
		return trueVal
	}
	return falseVal
}

// TestSuite represents the XML structure for test suites
type TestSuite struct {
	XMLName  xml.Name   `xml:"testsuite"`
	Name     string     `xml:"name,attr"`
	TestCase []TestCase `xml:"testcase"`
}

// TestCase represents the XML structure for a test case
type TestCase struct {
	Name    string  `xml:"name,attr"`
	Time    string  `xml:"time,attr"`
	Failure *string `xml:"failure"`
}

// TestSuites represents the XML structure for the root testsuites element
type TestSuites struct {
	XMLName   xml.Name    `xml:"testsuites"`
	TestSuite []TestSuite `xml:"testsuite"`
	Time      string      `xml:"time,attr"`
	Timestamp string      `xml:"timestamp,attr"`
}

// generateMarkdown reads the XML report, generates and prints the markdown report
func generateMarkdown(reportPath string, writer io.Writer) error {
	xmlFile, err := os.Open(reportPath)
	if err != nil {
		return fmt.Errorf("failed to open report file: %w", err)
	}
	defer xmlFile.Close()

	xmlData, err := io.ReadAll(xmlFile)
	if err != nil {
		return fmt.Errorf("failed to read report file: %w", err)
	}

	var testSuites TestSuites
	if err := xml.Unmarshal(xmlData, &testSuites); err != nil {
		return fmt.Errorf("failed to unmarshal XML: %w", err)
	}

	table := NewTable([]string{"Test Suite", "Test Case", "Time (s)", "Status"})

	for _, suite := range testSuites.TestSuite {
		for _, testcase := range suite.TestCase {
			status := Passed
			if testcase.Failure != nil {
				status = Failed
			}
			table.Add([]string{suite.Name, testcase.Name, fmt.Sprintf("`%s`", testcase.Time), status})
		}
	}

	report := NewReport(testSuites.Time, testSuites.Timestamp, table)
	report.Print(writer)

	return nil
}

// main function to run the report generator
func main() {
	reportPath := flag.String("file", "chainsaw-report.xml", "Path to XML report generated by Chainsaw")
	outputPath := flag.String("output", "", "Output file (defaults to stdout)")
	flag.Parse()

	var writer io.Writer = os.Stdout
	if *outputPath != "" {
		file, err := os.Create(*outputPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to open output file: %v\n", err)
			os.Exit(1)
		}
		defer file.Close()
		writer = file
	}

	if err := generateMarkdown(*reportPath, writer); err != nil {
		fmt.Fprintf(writer, "## Report generation failed :skull:\n\n")
		fmt.Fprintf(writer, "```log\n%v\n```\n", err)
		os.Exit(1)
	}
}
