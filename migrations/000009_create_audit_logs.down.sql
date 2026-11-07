DROP TRIGGER IF EXISTS trg_audit_logs_immutable ON audit_logs;
DROP FUNCTION IF EXISTS forbid_audit_log_mutation();
DROP TABLE IF EXISTS audit_logs CASCADE;
