package batch

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
)

// ParseURLList parses newline-separated URLs.
func ParseURLList(input string) ([]string, error) {
	scanner := bufio.NewScanner(strings.NewReader(input))
	urls := []string{}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		urls = append(urls, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan url list: %w", err)
	}
	return urls, nil
}

// ParseCSVURLs extracts URLs from the first column of a CSV file.
func ParseCSVURLs(reader io.Reader) ([]string, error) {
	csvReader := csv.NewReader(reader)
	urls := []string{}
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read csv: %w", err)
		}
		if len(record) == 0 {
			continue
		}
		url := strings.TrimSpace(record[0])
		if url != "" {
			urls = append(urls, url)
		}
	}
	return urls, nil
}
