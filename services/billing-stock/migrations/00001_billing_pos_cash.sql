-- Migration: 00001_billing_pos_cash.sql
-- Bounded Context: billing (POS, Electronic Invoicing RF-12, Blind Cash Close RN-02, Audit Trail RNF-06)

BEGIN;

CREATE TABLE IF NOT EXISTS cash_registers (
    id UUID PRIMARY KEY,
    clinic_id UUID NOT NULL,
    opened_by UUID NOT NULL,
    opened_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    opening_float_cents BIGINT NOT NULL CHECK (opening_float_cents >= 0),
    status VARCHAR(16) NOT NULL CHECK (status IN ('OPEN', 'CLOSED')),
    declared_cash_cents BIGINT CHECK (declared_cash_cents >= 0),
    theoretical_cash_cents BIGINT,
    discrepancy_cents BIGINT,
    denominations_json JSONB,
    cashier_notes TEXT,
    closed_by UUID,
    closed_at TIMESTAMPTZ,
    receipt_hash VARCHAR(128)
);

CREATE INDEX IF NOT EXISTS idx_cash_registers_clinic_status ON cash_registers (clinic_id, status);

CREATE TABLE IF NOT EXISTS cash_movements (
    id UUID PRIMARY KEY,
    cash_register_id UUID NOT NULL REFERENCES cash_registers(id) ON DELETE RESTRICT,
    invoice_id UUID,
    direction VARCHAR(8) NOT NULL CHECK (direction IN ('IN', 'OUT')),
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    reason VARCHAR(255) NOT NULL,
    actor_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_cash_movements_register_id ON cash_movements (cash_register_id);

CREATE TABLE IF NOT EXISTS invoices (
    id UUID PRIMARY KEY,
    patient_id UUID NOT NULL,
    cash_register_id UUID NOT NULL REFERENCES cash_registers(id) ON DELETE RESTRICT,
    customer_tax_id VARCHAR(64) NOT NULL,
    customer_name VARCHAR(255) NOT NULL,
    subtotal_cents BIGINT NOT NULL CHECK (subtotal_cents >= 0),
    tax_cents BIGINT NOT NULL CHECK (tax_cents >= 0),
    total_cents BIGINT NOT NULL CHECK (total_cents > 0),
    fiscal_authorization_code VARCHAR(128) NOT NULL UNIQUE,
    xml_hash VARCHAR(128) NOT NULL,
    stamped_at TIMESTAMPTZ NOT NULL,
    issued_by UUID NOT NULL,
    source_ip VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_invoices_patient_id ON invoices (patient_id);
CREATE INDEX IF NOT EXISTS idx_invoices_cash_register_id ON invoices (cash_register_id);

CREATE TABLE IF NOT EXISTS invoice_items (
    id UUID PRIMARY KEY,
    invoice_id UUID NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    procedure_code VARCHAR(64) NOT NULL,
    description TEXT NOT NULL,
    quantity INT NOT NULL CHECK (quantity > 0),
    unit_price_cents BIGINT NOT NULL CHECK (unit_price_cents >= 0),
    tax_rate_bps INT NOT NULL CHECK (tax_rate_bps >= 0 AND tax_rate_bps <= 10000),
    line_subtotal_cents BIGINT NOT NULL CHECK (line_subtotal_cents >= 0),
    line_tax_cents BIGINT NOT NULL CHECK (line_tax_cents >= 0),
    line_total_cents BIGINT NOT NULL CHECK (line_total_cents >= 0)
);

CREATE INDEX IF NOT EXISTS idx_invoice_items_invoice_id ON invoice_items (invoice_id);

CREATE TABLE IF NOT EXISTS invoice_payments (
    id UUID PRIMARY KEY,
    invoice_id UUID NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    method VARCHAR(32) NOT NULL CHECK (method IN ('CASH', 'CREDIT_CARD', 'DEBIT_CARD', 'BANK_TRANSFER', 'DENTAL_INSURANCE')),
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    external_reference VARCHAR(128) NOT NULL DEFAULT '',
    gateway_authorization VARCHAR(128) NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_invoice_payments_invoice_id ON invoice_payments (invoice_id);

CREATE TABLE IF NOT EXISTS financial_audit_logs (
    id UUID PRIMARY KEY,
    actor_id UUID NOT NULL,
    actor_role VARCHAR(32) NOT NULL,
    action VARCHAR(64) NOT NULL,
    resource_id VARCHAR(128) NOT NULL,
    timestamp_iso8601 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    source_ip VARCHAR(64) NOT NULL,
    details_json JSONB NOT NULL DEFAULT '{}'::jsonb
);

COMMIT;
