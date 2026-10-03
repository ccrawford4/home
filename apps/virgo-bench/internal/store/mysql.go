package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-sql-driver/mysql"
)

var schema = []string{
	`CREATE TABLE IF NOT EXISTS runs (
		id          BIGINT AUTO_INCREMENT PRIMARY KEY,
		model       VARCHAR(255) NOT NULL,
		status      VARCHAR(16)  NOT NULL,
		progress    VARCHAR(128) NOT NULL DEFAULT '',
		config      JSON         NOT NULL,
		summary     JSON         NULL,
		error       TEXT         NULL,
		created_at  DATETIME(3)  NOT NULL,
		started_at  DATETIME(3)  NULL,
		finished_at DATETIME(3)  NULL,
		INDEX idx_runs_model (model),
		INDEX idx_runs_status (status)
	)`,
	`CREATE TABLE IF NOT EXISTS requests (
		id              BIGINT AUTO_INCREMENT PRIMARY KEY,
		run_id          BIGINT       NOT NULL,
		phase           VARCHAR(16)  NOT NULL,
		seq             INT          NOT NULL,
		prompt          TEXT         NOT NULL,
		response        MEDIUMTEXT   NOT NULL,
		http_status     INT          NOT NULL,
		error           TEXT         NULL,
		client_ttft_ms  DOUBLE       NULL,
		client_ttlt_ms  DOUBLE       NULL,
		server_ttft_ms  DOUBLE       NULL,
		server_total_ms DOUBLE       NULL,
		eval_count      INT          NULL,
		decode_tps      DOUBLE       NULL,
		passed          BOOLEAN      NULL,
		tool_calls      JSON         NULL,
		chunks          JSON         NULL,
		started_at      DATETIME(3)  NOT NULL,
		INDEX idx_requests_run (run_id),
		CONSTRAINT fk_requests_run FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE CASCADE
	)`,
}

// MySQL is a Store backed by MySQL.
type MySQL struct{ db *sql.DB }

// MySQLConfig holds connection settings.
type MySQLConfig struct {
	Host, Port, User, Password, Database string
}

// OpenMySQL connects (waiting for the server to come up) and applies the schema.
func OpenMySQL(ctx context.Context, cfg MySQLConfig, log *slog.Logger) (*MySQL, error) {
	c := mysql.NewConfig()
	c.Net = "tcp"
	c.Addr = cfg.Host + ":" + cfg.Port
	c.User = cfg.User
	c.Passwd = cfg.Password
	c.DBName = cfg.Database
	c.ParseTime = true
	c.Loc = time.UTC
	db, err := sql.Open("mysql", c.FormatDSN())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	deadline := time.Now().Add(3 * time.Minute)
	for {
		err = db.PingContext(ctx)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			db.Close()
			return nil, fmt.Errorf("mysql not reachable: %w", err)
		}
		log.Warn("waiting for mysql", "error", err)
		select {
		case <-ctx.Done():
			db.Close()
			return nil, ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}
	for _, stmt := range schema {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			db.Close()
			return nil, fmt.Errorf("apply schema: %w", err)
		}
	}
	return &MySQL{db: db}, nil
}

func (s *MySQL) Close() error { return s.db.Close() }

const runCols = `id, model, status, progress, config, summary, error, created_at, started_at, finished_at`

func (s *MySQL) CreateRun(ctx context.Context, r *Run) error {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO runs (model, status, progress, config, summary, error, created_at, started_at, finished_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.Model, r.Status, r.Progress, jsonArg(r.Config), jsonArg(r.Summary), nullStr(r.Error), r.CreatedAt, r.StartedAt, r.FinishedAt)
	if err != nil {
		return err
	}
	r.ID, err = res.LastInsertId()
	return err
}

func (s *MySQL) UpdateRun(ctx context.Context, r *Run) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE runs SET status=?, progress=?, summary=?, error=?, started_at=?, finished_at=? WHERE id=?`,
		r.Status, r.Progress, jsonArg(r.Summary), nullStr(r.Error), r.StartedAt, r.FinishedAt, r.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		// MySQL reports 0 when nothing changed, so confirm the row exists.
		if _, err := s.GetRun(ctx, r.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *MySQL) GetRun(ctx context.Context, id int64) (*Run, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+runCols+` FROM runs WHERE id=?`, id)
	r, err := scanRun(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return r, err
}

func (s *MySQL) ListRuns(ctx context.Context, limit int) ([]Run, error) {
	if limit <= 0 {
		limit = 200
	}
	return s.queryRuns(ctx, `SELECT `+runCols+` FROM runs ORDER BY id DESC LIMIT ?`, limit)
}

func (s *MySQL) RunsWithStatus(ctx context.Context, status string) ([]Run, error) {
	return s.queryRuns(ctx, `SELECT `+runCols+` FROM runs WHERE status=? ORDER BY id ASC`, status)
}

func (s *MySQL) DeleteRun(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM runs WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *MySQL) queryRuns(ctx context.Context, q string, args ...any) ([]Run, error) {
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Run{}
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

type scanner interface{ Scan(dest ...any) error }

func scanRun(sc scanner) (*Run, error) {
	var r Run
	var config, summary []byte
	var errStr sql.NullString
	var started, finished sql.NullTime
	if err := sc.Scan(&r.ID, &r.Model, &r.Status, &r.Progress, &config, &summary, &errStr, &r.CreatedAt, &started, &finished); err != nil {
		return nil, err
	}
	r.Config = config
	if len(summary) > 0 {
		r.Summary = summary
	}
	r.Error = errStr.String
	if started.Valid {
		r.StartedAt = &started.Time
	}
	if finished.Valid {
		r.FinishedAt = &finished.Time
	}
	return &r, nil
}

func (s *MySQL) AddRequest(ctx context.Context, q *Request) error {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO requests (run_id, phase, seq, prompt, response, http_status, error,
			client_ttft_ms, client_ttlt_ms, server_ttft_ms, server_total_ms, eval_count, decode_tps,
			passed, tool_calls, chunks, started_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		q.RunID, q.Phase, q.Seq, q.Prompt, q.Response, q.HTTPStatus, nullStr(q.Error),
		q.ClientTTFTMs, q.ClientTTLTMs, q.ServerTTFTMs, q.ServerTotalMs, q.EvalCount, q.DecodeTPS,
		q.Passed, jsonArg(q.ToolCalls), jsonArg(q.Chunks), q.StartedAt)
	if err != nil {
		return err
	}
	q.ID, err = res.LastInsertId()
	return err
}

func (s *MySQL) ListRequests(ctx context.Context, runID int64) ([]Request, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, run_id, phase, seq, prompt, response, http_status, error,
			client_ttft_ms, client_ttlt_ms, server_ttft_ms, server_total_ms, eval_count, decode_tps,
			passed, tool_calls, chunks, started_at
		 FROM requests WHERE run_id=? ORDER BY id ASC`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Request{}
	for rows.Next() {
		var q Request
		var errStr sql.NullString
		var ttft, ttlt, sttft, stotal, tps sql.NullFloat64
		var evalCount sql.NullInt64
		var passed sql.NullBool
		var toolCalls, chunks []byte
		if err := rows.Scan(&q.ID, &q.RunID, &q.Phase, &q.Seq, &q.Prompt, &q.Response, &q.HTTPStatus, &errStr,
			&ttft, &ttlt, &sttft, &stotal, &evalCount, &tps, &passed, &toolCalls, &chunks, &q.StartedAt); err != nil {
			return nil, err
		}
		q.Error = errStr.String
		q.ClientTTFTMs, q.ClientTTLTMs = nf(ttft), nf(ttlt)
		q.ServerTTFTMs, q.ServerTotalMs, q.DecodeTPS = nf(sttft), nf(stotal), nf(tps)
		if evalCount.Valid {
			n := int(evalCount.Int64)
			q.EvalCount = &n
		}
		if passed.Valid {
			b := passed.Bool
			q.Passed = &b
		}
		if len(toolCalls) > 0 {
			q.ToolCalls = toolCalls
		}
		if len(chunks) > 0 {
			q.Chunks = chunks
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

func nf(v sql.NullFloat64) *float64 {
	if !v.Valid {
		return nil
	}
	return &v.Float64
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func jsonArg(b json.RawMessage) any {
	if len(b) == 0 {
		return nil
	}
	return string(b)
}
