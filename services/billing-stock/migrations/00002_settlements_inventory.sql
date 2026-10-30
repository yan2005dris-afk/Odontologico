-- Migration: 00002_settlements_inventory.sql
-- Bounded Contexts: settlements (RF-13) and inventory (RF-14)

BEGIN;

CREATE TABLE IF NOT EXISTS commission_rules (
    id UUID PRIMARY KEY,
    dentist_id UUID NOT NULL,
    procedure_code VARCHAR(64) NOT NULL,
    calculation_mode VARCHAR(32) NOT NULL CHECK (calculation_mode IN ('FIXED_PERCENTAGE', 'FIXED_AMOUNT', 'NET_PERCENTAGE_AFTER_COSTS')),
    rate_bps INT NOT NULL DEFAULT 0 CHECK (rate_bps >= 0 AND rate_bps <= 10000),
    fixed_amount_cents BIGINT NOT NULL DEFAULT 0 CHECK (fixed_amount_cents >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (dentist_id, procedure_code)
);

CREATE INDEX IF NOT EXISTS idx_commission_rules_dentist_id ON commission_rules (dentist_id);

CREATE TABLE IF NOT EXISTS dentist_settlements (
    id UUID PRIMARY KEY,
    dentist_id UUID NOT NULL,
    period_start DATE NOT NULL,
    period_end DATE NOT NULL,
    gross_fees_cents BIGINT NOT NULL CHECK (gross_fees_cents >= 0),
    total_lab_deductions_cents BIGINT NOT NULL CHECK (total_lab_deductions_cents >= 0),
    total_material_deductions_cents BIGINT NOT NULL CHECK (total_material_deductions_cents >= 0),
    net_payable_commission_cents BIGINT NOT NULL CHECK (net_payable_commission_cents >= 0),
    status VARCHAR(16) NOT NULL CHECK (status IN ('DRAFT', 'APPROVED', 'PAID')),
    generated_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_dentist_settlements_dentist_period ON dentist_settlements (dentist_id, period_start, period_end);

CREATE TABLE IF NOT EXISTS dentist_settlement_items (
    id UUID PRIMARY KEY,
    settlement_id UUID NOT NULL REFERENCES dentist_settlements(id) ON DELETE CASCADE,
    completed_procedure_id UUID NOT NULL UNIQUE,
    procedure_code VARCHAR(64) NOT NULL,
    gross_fee_cents BIGINT NOT NULL CHECK (gross_fee_cents >= 0),
    lab_cost_cents BIGINT NOT NULL CHECK (lab_cost_cents >= 0),
    material_cost_cents BIGINT NOT NULL CHECK (material_cost_cents >= 0),
    commission_cents BIGINT NOT NULL CHECK (commission_cents >= 0)
);

CREATE INDEX IF NOT EXISTS idx_dentist_settlement_items_settlement_id ON dentist_settlement_items (settlement_id);

CREATE TABLE IF NOT EXISTS inventory_supplies (
    id UUID PRIMARY KEY,
    clinic_id UUID NOT NULL,
    sku VARCHAR(64) NOT NULL,
    name VARCHAR(255) NOT NULL,
    unit VARCHAR(32) NOT NULL,
    batch_number VARCHAR(64) NOT NULL DEFAULT '',
    expiry_date DATE,
    current_stock INT NOT NULL CHECK (current_stock >= 0),
    min_stock_quantity INT NOT NULL CHECK (min_stock_quantity >= 0),
    unit_cost_cents BIGINT NOT NULL CHECK (unit_cost_cents >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (clinic_id, sku)
);

CREATE INDEX IF NOT EXISTS idx_inventory_supplies_clinic_sku ON inventory_supplies (clinic_id, sku);

CREATE TABLE IF NOT EXISTS procedure_supply_recipes (
    id UUID PRIMARY KEY,
    procedure_code VARCHAR(64) NOT NULL,
    supply_sku VARCHAR(64) NOT NULL,
    quantity_per_procedure INT NOT NULL CHECK (quantity_per_procedure > 0),
    UNIQUE (procedure_code, supply_sku)
);

CREATE INDEX IF NOT EXISTS idx_procedure_supply_recipes_code ON procedure_supply_recipes (procedure_code);

CREATE TABLE IF NOT EXISTS inventory_movements (
    id UUID PRIMARY KEY,
    clinic_id UUID NOT NULL,
    supply_id UUID NOT NULL REFERENCES inventory_supplies(id) ON DELETE RESTRICT,
    supply_sku VARCHAR(64) NOT NULL,
    completed_procedure_id UUID,
    movement_type VARCHAR(32) NOT NULL CHECK (movement_type IN ('PROCEDURE_CONSUMPTION', 'RESTOCK', 'ADJUSTMENT')),
    quantity_delta INT NOT NULL,
    remaining_stock INT NOT NULL CHECK (remaining_stock >= 0),
    unit_cost_cents BIGINT NOT NULL CHECK (unit_cost_cents >= 0),
    actor_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_inventory_movements_completed_proc ON inventory_movements (completed_procedure_id);

COMMIT;
