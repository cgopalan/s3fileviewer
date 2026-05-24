package duckdb

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/cgopalan/s3fileviewer/internal/files"
	_ "github.com/duckdb/duckdb-go/v2"
)

type Column struct {
	Name string
	Type string
}

type QueryResult struct {
	Columns   []string
	Rows      [][]string
	RowCount  int
	Truncated bool
}

type Pool struct {
	mu              sync.Mutex
	db              *sql.DB
	maxQueryRows    int
	queryTimeoutSec int
	awsRegion       string
	s3Configured    bool
}

func NewPool(maxQueryRows, queryTimeoutSec int, awsRegion string) (*Pool, error) {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		return nil, fmt.Errorf("open duckdb: %w", err)
	}

	p := &Pool{
		db:              db,
		maxQueryRows:    maxQueryRows,
		queryTimeoutSec: queryTimeoutSec,
		awsRegion:       awsRegion,
	}

	if err := p.init(); err != nil {
		db.Close()
		return nil, err
	}

	return p, nil
}

func (p *Pool) init() error {
	stmts := []string{
		"INSTALL httpfs",
		"LOAD httpfs",
	}
	for _, stmt := range stmts {
		if _, err := p.db.Exec(stmt); err != nil {
			return fmt.Errorf("duckdb init (%q): %w", stmt, err)
		}
	}
	return nil
}

func (p *Pool) ensureS3Secret() error {
	if p.s3Configured {
		return nil
	}

	_, err := p.db.Exec(`CREATE OR REPLACE SECRET s3_secret (
		TYPE s3,
		PROVIDER credential_chain
	)`)
	if err == nil {
		p.s3Configured = true
		return nil
	}

	keyID := strings.TrimSpace(os.Getenv("AWS_ACCESS_KEY_ID"))
	secret := strings.TrimSpace(os.Getenv("AWS_SECRET_ACCESS_KEY"))
	if keyID == "" || secret == "" {
		return fmt.Errorf("configure AWS credentials for DuckDB S3 access: %w", err)
	}

	sessionToken := strings.TrimSpace(os.Getenv("AWS_SESSION_TOKEN"))
	region := p.awsRegion
	if region == "" {
		region = strings.TrimSpace(os.Getenv("AWS_REGION"))
	}

	var stmt string
	if sessionToken != "" {
		stmt = fmt.Sprintf(`CREATE OR REPLACE SECRET s3_secret (
			TYPE s3,
			KEY_ID '%s',
			SECRET '%s',
			SESSION_TOKEN '%s',
			REGION '%s'
		)`, escapeSQLString(keyID), escapeSQLString(secret), escapeSQLString(sessionToken), escapeSQLString(region))
	} else {
		stmt = fmt.Sprintf(`CREATE OR REPLACE SECRET s3_secret (
			TYPE s3,
			KEY_ID '%s',
			SECRET '%s',
			REGION '%s'
		)`, escapeSQLString(keyID), escapeSQLString(secret), escapeSQLString(region))
	}

	if _, err := p.db.Exec(stmt); err != nil {
		return fmt.Errorf("create s3 secret: %w", err)
	}
	p.s3Configured = true
	return nil
}

func escapeSQLString(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func (p *Pool) Close() error {
	return p.db.Close()
}

func (p *Pool) registerView(bucket, key string) (files.FileReader, error) {
	if err := p.ensureS3Secret(); err != nil {
		return files.FileReader{}, err
	}
	reader := files.DetectReader(bucket, key)
	viewSQL := fmt.Sprintf("CREATE OR REPLACE VIEW data AS %s", reader.ViewSQL)
	if _, err := p.db.Exec(viewSQL); err != nil {
		return reader, fmt.Errorf("create view: %w", err)
	}
	return reader, nil
}

func (p *Pool) TryDescribe(bucket, key string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.ensureS3Secret(); err != nil {
		return err
	}

	reader := files.DetectReader(bucket, key)
	rows, err := p.db.Query(reader.DescribeSQL)
	if err != nil {
		return err
	}
	return rows.Close()
}

func (p *Pool) Schema(ctx context.Context, bucket, key string) ([]Column, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, time.Duration(p.queryTimeoutSec)*time.Second)
	defer cancel()

	if _, err := p.registerView(bucket, key); err != nil {
		return nil, err
	}

	rows, err := p.db.QueryContext(ctx, "DESCRIBE SELECT * FROM data")
	if err != nil {
		return nil, fmt.Errorf("describe: %w", err)
	}
	defer rows.Close()

	var colsOut []Column
	for rows.Next() {
		colNames, err := rows.Columns()
		if err != nil {
			return nil, fmt.Errorf("column names: %w", err)
		}
		vals := make([]interface{}, len(colNames))
		ptrs := make([]interface{}, len(colNames))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, fmt.Errorf("scan column: %w", err)
		}
		name := fmt.Sprintf("%v", vals[0])
		colType := ""
		if len(vals) > 1 {
			colType = fmt.Sprintf("%v", vals[1])
		}
		colsOut = append(colsOut, Column{Name: name, Type: colType})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return colsOut, nil
}

var (
	selectOnlyRe = regexp.MustCompile(`(?i)^\s*SELECT\b`)
	forbiddenRe  = regexp.MustCompile(`(?i)\b(DROP|DELETE|INSERT|UPDATE|CREATE|ALTER|COPY|ATTACH|INSTALL|LOAD|EXPORT|PRAGMA|SET)\b`)
	limitRe      = regexp.MustCompile(`(?i)\bLIMIT\b`)
)

func ValidateQuery(query string) error {
	q := strings.TrimSpace(query)
	if q == "" {
		return fmt.Errorf("query cannot be empty")
	}
	if strings.Contains(q, ";") {
		return fmt.Errorf("multi-statement queries are not allowed")
	}
	if !selectOnlyRe.MatchString(q) {
		return fmt.Errorf("only SELECT queries are allowed")
	}
	if forbiddenRe.MatchString(q) {
		return fmt.Errorf("query contains forbidden keywords")
	}
	return nil
}

func ensureLimit(query string, maxRows int) string {
	if limitRe.MatchString(query) {
		return query
	}
	return fmt.Sprintf("%s LIMIT %d", query, maxRows)
}

func (p *Pool) Query(ctx context.Context, bucket, key, query string) (*QueryResult, error) {
	if err := ValidateQuery(query); err != nil {
		return nil, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, time.Duration(p.queryTimeoutSec)*time.Second)
	defer cancel()

	if _, err := p.registerView(bucket, key); err != nil {
		return nil, err
	}

	limitedQuery := ensureLimit(query, p.maxQueryRows)
	rows, err := p.db.QueryContext(ctx, limitedQuery)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, fmt.Errorf("column types: %w", err)
	}
	columns := make([]string, len(colTypes))
	for i, ct := range colTypes {
		columns[i] = ct.Name()
	}

	var resultRows [][]string
	for rows.Next() {
		values := make([]interface{}, len(columns))
		ptrs := make([]interface{}, len(columns))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		row := make([]string, len(columns))
		for i, v := range values {
			if v == nil {
				row[i] = "NULL"
			} else {
				row[i] = fmt.Sprintf("%v", v)
			}
		}
		resultRows = append(resultRows, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	truncated := len(resultRows) >= p.maxQueryRows
	return &QueryResult{
		Columns:   columns,
		Rows:      resultRows,
		RowCount:  len(resultRows),
		Truncated: truncated,
	}, nil
}
