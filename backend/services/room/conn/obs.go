package conn

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
)

// dbTracer emits DEBUG-level logs for every Postgres statement executed by the
// service. It hooks into the pgx stdlib driver, so all database/sql (and sqlx)
// queries are captured.
type dbTracer struct {
	log *slog.Logger
}

var _ pgx.QueryTracer = (*dbTracer)(nil)

type dbTraceStartKey struct{}

type dbTraceInfo struct {
	sql   string
	args  []any
	start time.Time
}

func (t *dbTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, dbTraceStartKey{}, dbTraceInfo{sql: data.SQL, args: data.Args, start: time.Now()})
}

func (t *dbTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	if t.log == nil {
		return
	}
	info, _ := ctx.Value(dbTraceStartKey{}).(dbTraceInfo)
	t.log.Debug("db query",
		"sql", info.sql,
		"args", info.args,
		"rows", data.CommandTag.RowsAffected(),
		"duration", time.Since(info.start),
	)
	if data.Err != nil {
		t.log.Debug("db query error", "sql", info.sql, "error", data.Err)
	}
}

// newDB opens a *sqlx.DB backed by the pgx stdlib driver with the query tracer
// installed so every SQL statement is observable at DEBUG level.
func newDB(dsn string, log *slog.Logger) (*sqlx.DB, error) {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	if log != nil {
		cfg.Tracer = &dbTracer{log: log}
	}
	connector := stdlib.GetConnector(*cfg)
	return sqlx.NewDb(sql.OpenDB(connector), "pgx"), nil
}

// redisLogHook emits DEBUG-level logs for every Redis command, capturing the
// command name and primary key. Values are intentionally omitted to avoid
// leaking tokens or user data into the logs.
type redisLogHook struct {
	log *slog.Logger
}

var _ redis.Hook = (*redisLogHook)(nil)

func (h *redisLogHook) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (h *redisLogHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		if h.log == nil {
			return next(ctx, cmd)
		}
		start := time.Now()
		err := next(ctx, cmd)
		h.log.Debug("redis command",
			"cmd", cmd.Name(),
			"key", cmdKey(cmd.Args()),
			"duration", time.Since(start),
		)
		return err
	}
}

func (h *redisLogHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		if h.log == nil {
			return next(ctx, cmds)
		}
		start := time.Now()
		err := next(ctx, cmds)
		h.log.Debug("redis pipeline", "cmd_count", len(cmds), "duration", time.Since(start))
		return err
	}
}

// cmdKey returns the first argument after the command name (typically the key).
func cmdKey(args []any) any {
	if len(args) > 1 {
		return args[1]
	}
	return nil
}
