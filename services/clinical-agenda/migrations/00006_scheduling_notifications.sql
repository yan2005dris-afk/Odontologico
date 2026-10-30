-- 00006_scheduling_notifications.sql
-- Schema for multi-chair scheduling (RF-09), appointment state machine (RF-10), RN-03 chair overlap prevention, and 48h/24h notifications (RF-11).

CREATE EXTENSION IF NOT EXISTS "btree_gist";

CREATE TABLE IF NOT EXISTS dental_chairs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    clinic_id UUID NOT NULL,
    code VARCHAR(32) NOT NULL UNIQUE,
    name VARCHAR(120) NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE IF NOT EXISTS appointments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id UUID NOT NULL REFERENCES patients(id) ON DELETE CASCADE,
    dentist_id UUID NOT NULL,
    chair_id UUID NOT NULL REFERENCES dental_chairs(id) ON DELETE RESTRICT,
    clinic_id UUID NOT NULL,
    schedulable_task_id UUID,
    start_at TIMESTAMPTZ NOT NULL,
    end_at TIMESTAMPTZ NOT NULL CHECK (end_at > start_at),
    status VARCHAR(32) NOT NULL CHECK (status IN ('PROGRAMADO', 'CONFIRMADO', 'EN_SALA_DE_ESPERA', 'EN_ATENCION', 'ATENDIDO', 'CANCELADO', 'NO_SHOW')),
    reason TEXT NOT NULL,
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- RN-03: Strict physical dental chair overlap prevention regardless of dentist_id
    EXCLUDE USING gist (
        chair_id WITH =,
        tstzrange(start_at, end_at, '[)') WITH &&
    ) WHERE (status NOT IN ('CANCELADO', 'NO_SHOW'))
);

CREATE INDEX IF NOT EXISTS idx_appointments_chair_window ON appointments(chair_id, start_at, end_at);
CREATE INDEX IF NOT EXISTS idx_appointments_dentist_window ON appointments(dentist_id, start_at, end_at);

CREATE TABLE IF NOT EXISTS notification_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    appointment_id UUID NOT NULL REFERENCES appointments(id) ON DELETE CASCADE,
    reminder_type VARCHAR(32) NOT NULL CHECK (reminder_type IN ('CONFIRMATION_IMMEDIATE', 'REMINDER_48H', 'REMINDER_24H')),
    scheduled_for TIMESTAMPTZ NOT NULL,
    channels TEXT[] NOT NULL,
    status VARCHAR(32) NOT NULL CHECK (status IN ('PENDING', 'DISPATCHED', 'CANCELLED')),
    idempotency_key VARCHAR(255) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_notification_jobs_appointment ON notification_jobs(appointment_id, status);
