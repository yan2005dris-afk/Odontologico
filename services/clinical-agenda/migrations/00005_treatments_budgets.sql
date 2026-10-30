-- 00005_treatments_budgets.sql
-- Schema for multi-phase treatment plans & budgets (RF-07), partial/total approval into schedulable tasks (RF-08), and RN-01 immutability.

CREATE TABLE IF NOT EXISTS procedure_catalog (
    code VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    specialty VARCHAR(64) NOT NULL,
    base_fee_cents BIGINT NOT NULL CHECK (base_fee_cents >= 0),
    default_lab_cost_cents BIGINT NOT NULL DEFAULT 0 CHECK (default_lab_cost_cents >= 0),
    estimated_duration_minutes INT NOT NULL CHECK (estimated_duration_minutes BETWEEN 15 AND 240),
    requires_informed_consent BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE IF NOT EXISTS treatment_plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id UUID NOT NULL REFERENCES patients(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    currency VARCHAR(8) NOT NULL DEFAULT 'USD',
    status VARCHAR(32) NOT NULL CHECK (status IN ('DRAFT', 'PRESENTED', 'APPROVED_PARTIAL', 'APPROVED_TOTAL', 'COMPLETED')),
    total_gross_cents BIGINT NOT NULL DEFAULT 0,
    total_discount_cents BIGINT NOT NULL DEFAULT 0,
    total_net_cents BIGINT NOT NULL DEFAULT 0,
    approved_total_cents BIGINT NOT NULL DEFAULT 0,
    pending_total_cents BIGINT NOT NULL DEFAULT 0,
    patient_acceptance_ref VARCHAR(255),
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_treatment_plans_patient ON treatment_plans(patient_id, created_at DESC);

CREATE TABLE IF NOT EXISTS treatment_phases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_id UUID NOT NULL REFERENCES treatment_plans(id) ON DELETE CASCADE,
    phase_number INT NOT NULL CHECK (phase_number >= 1),
    name VARCHAR(255) NOT NULL,
    subtotal_net_cents BIGINT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS treatment_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    plan_id UUID NOT NULL REFERENCES treatment_plans(id) ON DELETE CASCADE,
    phase_number INT NOT NULL,
    procedure_code VARCHAR(64) NOT NULL,
    procedure_name VARCHAR(255) NOT NULL,
    tooth_fdi VARCHAR(8),
    surface VARCHAR(32),
    professional_fee_cents BIGINT NOT NULL CHECK (professional_fee_cents >= 0),
    lab_cost_cents BIGINT NOT NULL CHECK (lab_cost_cents >= 0),
    discount_cents BIGINT NOT NULL CHECK (discount_cents >= 0),
    net_cents BIGINT NOT NULL CHECK (net_cents >= 0),
    estimated_duration_minutes INT NOT NULL CHECK (estimated_duration_minutes BETWEEN 15 AND 240),
    status VARCHAR(32) NOT NULL CHECK (status IN ('PENDING_APPROVAL', 'APPROVED', 'REALIZADO')),
    completed_by UUID,
    completed_at TIMESTAMPTZ
);

CREATE OR REPLACE FUNCTION prevent_completed_treatment_item_mutation()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.status = 'REALIZADO' THEN
        RAISE EXCEPTION 'RN-01 violation: completed treatment items (REALIZADO) are strictly immutable';
    END IF;
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_prevent_completed_treatment_item_mutation ON treatment_items;
CREATE TRIGGER trg_prevent_completed_treatment_item_mutation
BEFORE UPDATE OR DELETE ON treatment_items
FOR EACH ROW EXECUTE FUNCTION prevent_completed_treatment_item_mutation();

CREATE TABLE IF NOT EXISTS schedulable_tasks (
    task_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id UUID NOT NULL REFERENCES patients(id) ON DELETE CASCADE,
    plan_id UUID NOT NULL REFERENCES treatment_plans(id) ON DELETE CASCADE,
    treatment_item_id UUID NOT NULL UNIQUE REFERENCES treatment_items(id) ON DELETE RESTRICT,
    phase_number INT NOT NULL,
    procedure_code VARCHAR(64) NOT NULL,
    procedure_name VARCHAR(255) NOT NULL,
    tooth_fdi VARCHAR(8),
    estimated_duration_minutes INT NOT NULL,
    status VARCHAR(32) NOT NULL CHECK (status IN ('READY_TO_SCHEDULE', 'SCHEDULED', 'COMPLETED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_schedulable_tasks_patient ON schedulable_tasks(patient_id, status);
