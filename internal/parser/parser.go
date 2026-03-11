// Reference: https://docs.aws.amazon.com/vpc/latest/userguide/flow-log-records.html
package parser

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/flowlog/service/internal/filter"
	"github.com/flowlog/service/internal/model"
)

const maxFileSize = 20 * 1024 * 1024 // 20 MB
var headerPattern = regexp.MustCompile(`^[a-z][a-z0-9\-]*$`)

type Parser struct{}

// Constructor
func New() *Parser { return &Parser{} }

func (p *Parser) Parse(logPath string, criteria *filter.Criteria) (*model.ParseResult, error) {
	if err := validateFile(logPath); err != nil {
		return nil, err
	}

	f, err := os.Open(logPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open file: %w", err)
	}
	defer f.Close()

	return parseStream(f, criteria)
}

func (p *Parser) ParseReader(r io.Reader, criteria *filter.Criteria) (*model.ParseResult, error) {
	return parseStream(r, criteria)
}

var ErrFileNotFound = errors.New("file not found")
var ErrFileTooLarge = errors.New("file bigger than 20 MB")
var ErrNotRegularFile = errors.New("irregular file")
var ErrNonASCII = errors.New("file has non-ASCII bytes")

func validateFile(path string) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return ErrFileNotFound
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return ErrNotRegularFile
	}
	if info.Size() > maxFileSize {
		return fmt.Errorf("%w (got %d bytes)", ErrFileTooLarge, info.Size())
	}

	// ASCII check
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	buf := make([]byte, 65536)
	for {
		n, rerr := f.Read(buf)
		for i := 0; i < n; i++ {
			if buf[i] > 127 {
				return ErrNonASCII
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
	}
	return nil
}

func ValidateReader(r io.Reader, maxBytes int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("%w (limit %d bytes)", ErrFileTooLarge, maxBytes)
	}
	for _, b := range data {
		if b > 127 {
			return nil, ErrNonASCII
		}
	}
	return data, nil
}

func parseStream(r io.Reader, criteria *filter.Criteria) (*model.ParseResult, error) {
	result := &model.ParseResult{
		ConnectionCounts: make(map[model.ConnectionKey]int64),
	}

	var columns []string
	var hasHeader bool
	headerDetected := false

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1 MB line buffer
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if !headerDetected {
			headerDetected = true
			columns, hasHeader = detectFormat(line)
			if hasHeader {
				continue
			}
		}

		fields := strings.Fields(line)

		if hasHeader && isHeaderLine(fields) {
			continue
		}

		for len(fields) < len(columns) {
			fields = append(fields, model.FieldNotApplicable)
		}

		fieldMap := make(map[string]string, len(columns))
		for i, col := range columns {
			fieldMap[col] = fields[i]
		}

		rec := &model.Record{
			LineNumber: lineNum,
			Columns:    columns,
			Fields:     fieldMap,
		}
		result.TotalRecords++

		if ver := rec.Get("version"); ver != model.FieldNotApplicable {
			if !isNumeric(ver) {
				result.SkippedRecords++
				continue
			}
		}

		if criteria != nil && !criteria.Matches(rec) {
			continue
		}

		result.MatchedRecords = append(result.MatchedRecords, rec)

		key := model.ConnectionKeyFrom(rec)
		if _, seen := result.ConnectionCounts[key]; !seen {
			result.ConnectionOrder = append(result.ConnectionOrder, key)
		}
		result.ConnectionCounts[key]++
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading log file: %w", err)
	}

	return result, nil
}

func detectFormat(firstLine string) (columns []string, isHeader bool) {
	parts := strings.Fields(firstLine)
	if len(parts) == 0 {
		return model.DefaultV2Columns, false
	}
	if headerPattern.MatchString(parts[0]) {
		return parts, true
	}
	return model.DefaultV2Columns, false
}

func isHeaderLine(fields []string) bool {
	if len(fields) == 0 {
		return false
	}
	return headerPattern.MatchString(fields[0])
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
