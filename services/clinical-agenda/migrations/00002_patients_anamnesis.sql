-- 00002_patients_anamnesis.sql
-- Schema for unified patient demographic records (RF-01), versioned anamnesis & critical alerts (RF-02), and fast search indexes (RNF-01).

CREATE TABLE IF NOT EXISTS patients (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    hc_number VARCHAR(32) NOT NULL UNIQUE,
    document_type VARCHAR(32) NOT NULL CHECK (document_type IN ('DNI', 'PASSPORT', 'FOREIGN_ID')),
    document_number VARCHAR(32) NOT NULL UNIQUE,
    first_name VARCHAR(120) NOT NULL,
    last_name VARCHAR(120) NOT NULL,
    birth_date DATE NOT NULL,
    sex VARCHAR(8) NOT NULL CHECK (sex IN ('F', 'M', 'X')),
    phone VARCHAR(64) NOT NULL,
    email VARCHAR(255),
    emergency_contact_json JSONB NOT NULL,
    legal_guardian_json JSONB,
    insurance_plan_json JSONB,
    anamnesis_recorded BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_patients_document ON patients(document_number);
CREATE INDEX IF NOT EXISTS idx_patients_hc_number ON patients(hc_number);
CREATE INDEX IF NOT EXISTS idx_patients_fullname ON patients(LOWER(last_name), LOWER(first_name));

CREATE TABLE IF NOT EXISTS patient_anamnesis (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id UUID NOT NULL REFERENCES patients(id) ON DELETE CASCADE,
    systemic_diseases_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    drug_allergies_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    surgical_history_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    coagulopathies_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    is_pregnant BOOLEAN NOT NULL DEFAULT FALSE,
    notes TEXT NOT NULL DEFAULT '',
    recorded_by UUID NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_patient_anamnesis_patient ON patient_anamnesis(patient_id, recorded_at DESC);

CREATE TABLE IF NOT EXISTS patient_critical_alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id UUID NOT NULL REFERENCES patients(id) ON DELETE CASCADE,
    code VARCHAR(64) NOT NULL,
    label VARCHAR(255) NOT NULL,
    severity VARCHAR(32) NOT NULL CHECK (severity IN ('CRITICAL', 'HIGH')),
    category VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_patient_critical_alerts_patient ON patient_critical_alerts(patient_id);
