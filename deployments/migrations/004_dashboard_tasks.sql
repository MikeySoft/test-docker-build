-- Dashboard tasks table for fleet health actions

CREATE TABLE IF NOT EXISTS dashboard_tasks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    title VARCHAR(255) NOT NULL,
    description TEXT,
    status VARCHAR(32) NOT NULL DEFAULT 'open',
    severity VARCHAR(32) NOT NULL DEFAULT 'info',
    source VARCHAR(32) NOT NULL DEFAULT 'system',
    category VARCHAR(64),
    task_type VARCHAR(64),
    fingerprint VARCHAR(255),
    host_id UUID REFERENCES hosts(id) ON DELETE SET NULL,
    stack_id UUID REFERENCES stacks(id) ON DELETE SET NULL,
    container_id VARCHAR(128),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    due_at TIMESTAMP,
    snoozed_until TIMESTAMP,
    acknowledged_at TIMESTAMP,
    resolved_at TIMESTAMP,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    acknowledged_by UUID REFERENCES users(id) ON DELETE SET NULL,
    resolved_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_dashboard_tasks_status ON dashboard_tasks(status);
CREATE INDEX IF NOT EXISTS idx_dashboard_tasks_severity ON dashboard_tasks(severity);
CREATE INDEX IF NOT EXISTS idx_dashboard_tasks_source ON dashboard_tasks(source);
CREATE INDEX IF NOT EXISTS idx_dashboard_tasks_host ON dashboard_tasks(host_id);
CREATE INDEX IF NOT EXISTS idx_dashboard_tasks_stack ON dashboard_tasks(stack_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_dashboard_tasks_fingerprint_active
    ON dashboard_tasks(fingerprint)
    WHERE fingerprint IS NOT NULL AND status IN ('open', 'acknowledged');

