-- 00004_soep_consents.sql
-- Schema for immutable SOEP evolution notes locked after 24h (RF-05, RN-01) and digital informed consents (RF-06).

CREATE TABLE IF NOT EXISTS soep_notes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id UUID NOT NULL REFERENCES patients(id) ON DELETE CASCADE,
    appointment_id UUID,
    procedure_code VARCHAR(64),
    author_dentist_id UUID NOT NULL,
    subjective TEXT NOT NULL,
    objective TEXT NOT NULL,
    assessment TEXT NOT NULL,
    plan TEXT NOT NULL,
    signature_hash VARCHAR(128) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    locks_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_soep_notes_patient ON soep_notes(patient_id, created_at DESC);

CREATE OR REPLACE FUNCTION enforce_soep_note_immutability()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'RN-01 violation: SOEP notes can never be deleted';
    END IF;
    IF TG_OP = 'UPDATE' AND NOW() >= OLD.locks_at THEN
        RAISE EXCEPTION 'RN-01 violation: SOEP note is locked past its 24h window; submit a signed addendum';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_enforce_soep_note_immutability ON soep_notes;
CREATE TRIGGER trg_enforce_soep_note_immutability
BEFORE UPDATE OR DELETE ON soep_notes
FOR EACH ROW EXECUTE FUNCTION enforce_soep_note_immutability();

CREATE TABLE IF NOT EXISTS soep_addenda (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    parent_note_id UUID NOT NULL REFERENCES soep_notes(id) ON DELETE RESTRICT,
    author_dentist_id UUID NOT NULL,
    reason TEXT NOT NULL,
    correction_text TEXT NOT NULL,
    signature_hash VARCHAR(128) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_soep_addenda_parent ON soep_addenda(parent_note_id, created_at ASC);

CREATE TABLE IF NOT EXISTS consent_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    procedure_code VARCHAR(64) NOT NULL UNIQUE,
    title VARCHAR(255) NOT NULL,
    body_markdown TEXT NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS patient_consents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id UUID NOT NULL REFERENCES patients(id) ON DELETE CASCADE,
    procedure_code VARCHAR(64) NOT NULL,
    title VARCHAR(255) NOT NULL,
    rendered_text TEXT NOT NULL,
    status VARCHAR(32) NOT NULL CHECK (status IN ('PENDING_SIGNATURE', 'SIGNED', 'REVOKED')),
    signature_method VARCHAR(32) CHECK (signature_method IN ('BIOMETRIC', 'OTP')),
    signed_by_name VARCHAR(255),
    signed_by_document VARCHAR(64),
    document_sha256 VARCHAR(64),
    storage_object_key VARCHAR(512),
    signed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_patient_consents_patient ON patient_consents(patient_id, created_at DESC);
