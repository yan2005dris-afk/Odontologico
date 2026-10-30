-- 00003_odontogram_perio.sql
-- Schema for interactive 2D odontogram (RF-03, RN-01) and digital periodontogram (RF-04).

CREATE TABLE IF NOT EXISTS odontogram_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id UUID NOT NULL REFERENCES patients(id) ON DELETE CASCADE,
    dentition_mode VARCHAR(32) NOT NULL DEFAULT 'MIXED',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS odontogram_findings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id UUID NOT NULL REFERENCES patients(id) ON DELETE CASCADE,
    tooth_fdi VARCHAR(4) NOT NULL,
    tooth_universal VARCHAR(4) NOT NULL,
    dentition_type VARCHAR(16) NOT NULL CHECK (dentition_type IN ('PERMANENT', 'DECIDUOUS')),
    surface VARCHAR(32) NOT NULL CHECK (surface IN ('MESIAL', 'DISTAL', 'OCLUSAL_INCISAL', 'VESTIBULAR', 'LINGUAL_PALATINO', 'WHOLE_TOOTH')),
    finding_type VARCHAR(32) NOT NULL,
    color_hex VARCHAR(16) NOT NULL,
    status VARCHAR(16) NOT NULL CHECK (status IN ('DIAGNOSED', 'PLANNED', 'REALIZADO')),
    notes TEXT NOT NULL DEFAULT '',
    recorded_by UUID NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_odontogram_findings_patient ON odontogram_findings(patient_id, recorded_at DESC);

CREATE OR REPLACE FUNCTION prevent_completed_finding_mutation()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.status = 'REALIZADO' THEN
        RAISE EXCEPTION 'RN-01 violation: completed odontogram findings (REALIZADO) are strictly immutable';
    END IF;
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_prevent_completed_finding_mutation ON odontogram_findings;
CREATE TRIGGER trg_prevent_completed_finding_mutation
BEFORE UPDATE OR DELETE ON odontogram_findings
FOR EACH ROW EXECUTE FUNCTION prevent_completed_finding_mutation();

CREATE TABLE IF NOT EXISTS periodontogram_charts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id UUID NOT NULL REFERENCES patients(id) ON DELETE CASCADE,
    bop_percentage NUMERIC(5,2) NOT NULL,
    mean_nic_mm NUMERIC(5,2) NOT NULL,
    recorded_by UUID NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_periodontogram_charts_patient ON periodontogram_charts(patient_id, recorded_at DESC);

CREATE TABLE IF NOT EXISTS periodontogram_sites (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chart_id UUID NOT NULL REFERENCES periodontogram_charts(id) ON DELETE CASCADE,
    tooth_fdi VARCHAR(4) NOT NULL,
    probing_depth_mm INT[] NOT NULL,
    gingival_margin_mm INT[] NOT NULL,
    nic_mm INT[] NOT NULL,
    bleeding_on_probing BOOLEAN[] NOT NULL,
    furcation_grade INT NOT NULL CHECK (furcation_grade BETWEEN 0 AND 3),
    mobility_grade INT NOT NULL CHECK (mobility_grade BETWEEN 0 AND 3)
);
