package warmup

// This additive, transactional index keeps existing execution JSON and dedup keys
// intact. Triggers also cover scheduler writes and older compatible binaries.
func (s *Store) initHistoryIndex() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var exists int
	if err = tx.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='history_origins'").Scan(&exists); err != nil {
		return err
	}
	_, err = tx.Exec(`CREATE TABLE IF NOT EXISTS history_origins(execution_id TEXT NOT NULL,run_id TEXT NOT NULL,active INTEGER NOT NULL,PRIMARY KEY(execution_id,run_id));
 CREATE INDEX IF NOT EXISTS history_origins_run ON history_origins(run_id,execution_id);
 CREATE INDEX IF NOT EXISTS history_runs_started ON runs(started DESC,id DESC);
 CREATE INDEX IF NOT EXISTS history_runs_task ON runs(task_id,started DESC,id DESC);
 CREATE INDEX IF NOT EXISTS history_executions_status ON executions(json_extract(body,'$.status'));
 CREATE INDEX IF NOT EXISTS history_executions_started ON executions(json_extract(body,'$.started') DESC,id DESC);
 CREATE INDEX IF NOT EXISTS history_executions_finished ON executions(json_extract(body,'$.finished'));
 CREATE INDEX IF NOT EXISTS history_executions_auth ON executions(json_extract(body,'$.authId'),json_extract(body,'$.finished'));
 CREATE TRIGGER IF NOT EXISTS history_execution_insert AFTER INSERT ON executions BEGIN
 INSERT OR REPLACE INTO history_origins SELECT NEW.id,json_extract(value,'$.runId'),coalesce(json_extract(value,'$.active'),0) FROM json_each(NEW.body,'$.origins') WHERE json_extract(value,'$.runId') IS NOT NULL; END;
 CREATE TRIGGER IF NOT EXISTS history_execution_update AFTER UPDATE OF body ON executions BEGIN
 DELETE FROM history_origins WHERE execution_id=NEW.id;
 INSERT OR REPLACE INTO history_origins SELECT NEW.id,json_extract(value,'$.runId'),coalesce(json_extract(value,'$.active'),0) FROM json_each(NEW.body,'$.origins') WHERE json_extract(value,'$.runId') IS NOT NULL; END;
 CREATE TRIGGER IF NOT EXISTS history_execution_delete AFTER DELETE ON executions BEGIN DELETE FROM history_origins WHERE execution_id=OLD.id; END;`)
	if err != nil {
		return err
	}
	if exists == 0 {
		_, err = tx.Exec(`INSERT OR REPLACE INTO history_origins SELECT e.id,json_extract(o.value,'$.runId'),coalesce(json_extract(o.value,'$.active'),0) FROM executions e,json_each(e.body,'$.origins') o WHERE json_extract(o.value,'$.runId') IS NOT NULL`)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
