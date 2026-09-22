package sqlitestore

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS rules (
  id TEXT PRIMARY KEY, owner TEXT NOT NULL, symbol TEXT NOT NULL,
  max_premium_bps INTEGER NOT NULL, budget_micro INTEGER NOT NULL,
  max_impact_bps INTEGER NOT NULL, cooldown_sec INTEGER NOT NULL,
  enabled INTEGER NOT NULL, created_ms INTEGER NOT NULL, last_triggered_ms INTEGER NOT NULL);
CREATE INDEX IF NOT EXISTS idx_rules_owner ON rules(owner, created_ms, id);
CREATE INDEX IF NOT EXISTS idx_rules_enabled ON rules(enabled, created_ms, id);

CREATE TABLE IF NOT EXISTS snapshots (
  symbol TEXT NOT NULL, observed_ms INTEGER NOT NULL, name TEXT NOT NULL, description TEXT NOT NULL,
  mint TEXT NOT NULL, token_price REAL NOT NULL, mark_price REAL NOT NULL, supply REAL NOT NULL,
  mark_valuation REAL NOT NULL, implied_valuation REAL NOT NULL);
CREATE INDEX IF NOT EXISTS idx_snapshots_symbol_time ON snapshots(symbol, observed_ms);

CREATE TABLE IF NOT EXISTS signals (
  id TEXT PRIMARY KEY, rule_id TEXT NOT NULL, owner TEXT NOT NULL, symbol TEXT NOT NULL,
  token_price REAL NOT NULL, mark_price REAL NOT NULL, api_premium_bps REAL NOT NULL, exec_premium_bps REAL NOT NULL,
  q_input_mint TEXT NOT NULL, q_output_mint TEXT NOT NULL, q_in_micro INTEGER NOT NULL, q_out_tokens REAL NOT NULL,
  q_impact_bps REAL NOT NULL, q_swap_url TEXT NOT NULL, q_quoted_ms INTEGER NOT NULL, created_ms INTEGER NOT NULL);
CREATE INDEX IF NOT EXISTS idx_signals_owner_time ON signals(owner, created_ms, id);
`

func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}

	db.SetMaxOpenConns(1)

	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;`); err != nil {
		db.Close()
		return nil, fmt.Errorf("init sqlite pragmas: %w", err)
	}

	if _, err := db.ExecContext(ctx, schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("init sqlite schema: %w", err)
	}

	return db, nil
}
