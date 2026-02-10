package config

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// ErrDatabaseNotConfigured is returned when database operations are attempted without configuration
var ErrDatabaseNotConfigured = fmt.Errorf("database is not configured")

// DBClient wraps PostgreSQL direct connection
type DBClient struct {
	postgres *sql.DB
}

// DBClients holds admin and public database clients
type DBClients struct {
	Admin  *DBClient
	Public *DBClient
}

// NewDBClients creates database clients based on configuration
func NewDBClients(cfg *Config) (*DBClients, error) {
	if !cfg.HasPostgres() {
		return nil, ErrDatabaseNotConfigured
	}

	pgCfg := cfg.GetPostgresConfig()
	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		pgCfg.Host, pgCfg.Port, pgCfg.User, pgCfg.Password, pgCfg.Database, pgCfg.SSLMode,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DBClients{
		Admin:  &DBClient{postgres: db},
		Public: &DBClient{postgres: db},
	}, nil
}

// IsMock returns true if this is a mock client (always false for PostgreSQL)
func (c *DBClient) IsMock() bool {
	return c.postgres == nil
}

// Close closes the database connection
func (c *DBClient) Close() error {
	if c.postgres != nil {
		return c.postgres.Close()
	}
	return nil
}

// DBQueryBuilder wraps query building for PostgreSQL
type DBQueryBuilder struct {
	client     *DBClient
	table      string
	selectCols string
	filters    []string
	args       []interface{}
	order      string
	limit      int
	offset     int
	single     bool
}

// From starts a query on a table
func (c *DBClient) From(table string) *DBQueryBuilder {
	return &DBQueryBuilder{
		client: c,
		table:  table,
		args:   make([]interface{}, 0),
	}
}

// Select specifies columns to return
func (q *DBQueryBuilder) Select(cols string) *DBQueryBuilder {
	q.selectCols = cols
	return q
}

// Eq adds an equality filter
func (q *DBQueryBuilder) Eq(column, value string) *DBQueryBuilder {
	q.args = append(q.args, value)
	q.filters = append(q.filters, fmt.Sprintf("%s = $%d", column, len(q.args)))
	return q
}

// Neq adds a not-equal filter
func (q *DBQueryBuilder) Neq(column, value string) *DBQueryBuilder {
	q.args = append(q.args, value)
	q.filters = append(q.filters, fmt.Sprintf("%s != $%d", column, len(q.args)))
	return q
}

// In adds an IN filter
func (q *DBQueryBuilder) In(column string, values []string) *DBQueryBuilder {
	placeholders := make([]string, len(values))
	for i, v := range values {
		q.args = append(q.args, v)
		placeholders[i] = fmt.Sprintf("$%d", len(q.args))
	}
	q.filters = append(q.filters, fmt.Sprintf("%s IN (%s)", column, strings.Join(placeholders, ",")))
	return q
}

// Is adds an IS filter (for null checks)
func (q *DBQueryBuilder) Is(column, value string) *DBQueryBuilder {
	if value == "null" {
		q.filters = append(q.filters, fmt.Sprintf("%s IS NULL", column))
	} else {
		q.filters = append(q.filters, fmt.Sprintf("%s IS NOT NULL", column))
	}
	return q
}

// Order sets the order
func (q *DBQueryBuilder) Order(column string, ascending bool) *DBQueryBuilder {
	if ascending {
		q.order = column + " ASC"
	} else {
		q.order = column + " DESC"
	}
	return q
}

// Limit sets the limit
func (q *DBQueryBuilder) Limit(n int) *DBQueryBuilder {
	q.limit = n
	return q
}

// Offset sets the offset
func (q *DBQueryBuilder) Offset(n int) *DBQueryBuilder {
	q.offset = n
	return q
}

// Single expects a single result
func (q *DBQueryBuilder) Single() *DBQueryBuilder {
	q.single = true
	q.limit = 1
	return q
}

// Execute runs the query and unmarshals into dest
func (q *DBQueryBuilder) Execute(ctx context.Context, dest interface{}) error {
	if q.client.IsMock() {
		return ErrDatabaseNotConfigured
	}

	cols := "*"
	if q.selectCols != "" {
		cols = q.selectCols
	}

	query := fmt.Sprintf("SELECT %s FROM %s", cols, q.table)

	if len(q.filters) > 0 {
		query += " WHERE " + strings.Join(q.filters, " AND ")
	}
	if q.order != "" {
		query += " ORDER BY " + q.order
	}
	if q.limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", q.limit)
	}
	if q.offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", q.offset)
	}

	rows, err := q.client.postgres.QueryContext(ctx, query, q.args...)
	if err != nil {
		return fmt.Errorf("query error: %w", err)
	}
	defer func() { _ = rows.Close() }()

	columns, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("columns error: %w", err)
	}

	var results []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return fmt.Errorf("scan error: %w", err)
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("rows error: %w", err)
	}

	jsonData, err := json.Marshal(results)
	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}

	return json.Unmarshal(jsonData, dest)
}

// DBInsertBuilder wraps insert building for PostgreSQL
type DBInsertBuilder struct {
	client     *DBClient
	table      string
	data       interface{}
	onConflict string
	returning  string
	upsert     bool
}

// Insert starts an insert operation
func (c *DBClient) Insert(table string, data interface{}) *DBInsertBuilder {
	return &DBInsertBuilder{
		client: c,
		table:  table,
		data:   data,
	}
}

// Upsert starts an upsert operation
func (c *DBClient) Upsert(table string, data interface{}) *DBInsertBuilder {
	return &DBInsertBuilder{
		client: c,
		table:  table,
		data:   data,
		upsert: true,
	}
}

// OnConflict sets the conflict resolution
func (b *DBInsertBuilder) OnConflict(columns string) *DBInsertBuilder {
	b.onConflict = columns
	return b
}

// Returning sets which columns to return
func (b *DBInsertBuilder) Returning(cols string) *DBInsertBuilder {
	b.returning = cols
	return b
}

// Execute runs the insert
func (b *DBInsertBuilder) Execute(ctx context.Context, dest interface{}) error {
	if b.client.IsMock() {
		return ErrDatabaseNotConfigured
	}

	jsonData, err := json.Marshal(b.data)
	if err != nil {
		return err
	}

	var dataMap map[string]interface{}
	if err := json.Unmarshal(jsonData, &dataMap); err != nil {
		return err
	}

	columns := make([]string, 0, len(dataMap))
	placeholders := make([]string, 0, len(dataMap))
	args := make([]interface{}, 0, len(dataMap))

	i := 1
	for col, val := range dataMap {
		columns = append(columns, col)
		placeholders = append(placeholders, fmt.Sprintf("$%d", i))
		args = append(args, val)
		i++
	}

	query := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		b.table,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)

	if b.upsert && b.onConflict != "" {
		updateClauses := make([]string, 0, len(columns))
		for _, col := range columns {
			updateClauses = append(updateClauses, fmt.Sprintf("%s = EXCLUDED.%s", col, col))
		}
		query += fmt.Sprintf(" ON CONFLICT (%s) DO UPDATE SET %s", b.onConflict, strings.Join(updateClauses, ", "))
	}

	if b.returning != "" {
		query += " RETURNING " + b.returning
	}

	if dest != nil && b.returning != "" {
		rows, err := b.client.postgres.QueryContext(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("insert query error: %w", err)
		}
		defer func() { _ = rows.Close() }()

		columns, err := rows.Columns()
		if err != nil {
			return err
		}

		var results []map[string]interface{}
		for rows.Next() {
			values := make([]interface{}, len(columns))
			valuePtrs := make([]interface{}, len(columns))
			for i := range values {
				valuePtrs[i] = &values[i]
			}

			if err := rows.Scan(valuePtrs...); err != nil {
				return err
			}

			row := make(map[string]interface{})
			for i, col := range columns {
				val := values[i]
				if bytes, ok := val.([]byte); ok {
					row[col] = string(bytes)
				} else {
					row[col] = val
				}
			}
			results = append(results, row)
		}

		jsonData, err := json.Marshal(results)
		if err != nil {
			return err
		}
		return json.Unmarshal(jsonData, dest)
	}

	_, err = b.client.postgres.ExecContext(ctx, query, args...)
	return err
}

// DBUpdateBuilder wraps update building for PostgreSQL
type DBUpdateBuilder struct {
	client  *DBClient
	table   string
	data    interface{}
	filters []string
	args    []interface{}
}

// Update starts an update operation
func (c *DBClient) Update(table string, data interface{}) *DBUpdateBuilder {
	return &DBUpdateBuilder{
		client: c,
		table:  table,
		data:   data,
		args:   make([]interface{}, 0),
	}
}

// Eq adds an equality filter
func (b *DBUpdateBuilder) Eq(column, value string) *DBUpdateBuilder {
	b.args = append(b.args, value)
	b.filters = append(b.filters, fmt.Sprintf("%s = $%d", column, len(b.args)))
	return b
}

// Execute runs the update
func (b *DBUpdateBuilder) Execute(ctx context.Context, dest interface{}) error {
	if b.client.IsMock() {
		return ErrDatabaseNotConfigured
	}

	jsonData, err := json.Marshal(b.data)
	if err != nil {
		return err
	}

	var dataMap map[string]interface{}
	if err := json.Unmarshal(jsonData, &dataMap); err != nil {
		return err
	}

	setClauses := make([]string, 0, len(dataMap))
	startIdx := len(b.args) + 1
	for col, val := range dataMap {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", col, startIdx))
		b.args = append(b.args, val)
		startIdx++
	}

	query := fmt.Sprintf("UPDATE %s SET %s", b.table, strings.Join(setClauses, ", "))

	if len(b.filters) > 0 {
		query += " WHERE " + strings.Join(b.filters, " AND ")
	}

	_, err = b.client.postgres.ExecContext(ctx, query, b.args...)
	return err
}

// DBDeleteBuilder wraps delete building for PostgreSQL
type DBDeleteBuilder struct {
	client  *DBClient
	table   string
	filters []string
	args    []interface{}
}

// Delete starts a delete operation
func (c *DBClient) Delete(table string) *DBDeleteBuilder {
	return &DBDeleteBuilder{
		client: c,
		table:  table,
		args:   make([]interface{}, 0),
	}
}

// Eq adds an equality filter
func (b *DBDeleteBuilder) Eq(column, value string) *DBDeleteBuilder {
	b.args = append(b.args, value)
	b.filters = append(b.filters, fmt.Sprintf("%s = $%d", column, len(b.args)))
	return b
}

// Execute runs the delete
func (b *DBDeleteBuilder) Execute(ctx context.Context) error {
	if b.client.IsMock() {
		return ErrDatabaseNotConfigured
	}

	query := fmt.Sprintf("DELETE FROM %s", b.table)
	if len(b.filters) > 0 {
		query += " WHERE " + strings.Join(b.filters, " AND ")
	}
	_, err := b.client.postgres.ExecContext(ctx, query, b.args...)
	return err
}

// RPC calls a database function
func (c *DBClient) RPC(ctx context.Context, function string, params interface{}, dest interface{}) error {
	if c.IsMock() {
		return ErrDatabaseNotConfigured
	}

	var paramsMap map[string]interface{}
	if params != nil {
		jsonData, err := json.Marshal(params)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(jsonData, &paramsMap); err != nil {
			return err
		}
	}

	argNames := make([]string, 0, len(paramsMap))
	args := make([]interface{}, 0, len(paramsMap))
	i := 1
	for name, val := range paramsMap {
		argNames = append(argNames, fmt.Sprintf("%s := $%d", name, i))
		args = append(args, val)
		i++
	}

	query := fmt.Sprintf("SELECT * FROM %s(%s)", function, strings.Join(argNames, ", "))

	rows, err := c.postgres.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("rpc error: %w", err)
	}
	defer func() { _ = rows.Close() }()

	if dest == nil {
		return nil
	}

	columns, err := rows.Columns()
	if err != nil {
		return err
	}

	var results []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return err
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			if bytes, ok := val.([]byte); ok {
				row[col] = string(bytes)
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
	}

	jsonData, err := json.Marshal(results)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonData, dest)
}
