package domain

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidToothCode        = errors.New("tooth code is outside the 52 canonical FDI/Universal teeth")
	ErrInvalidNotationSystem   = errors.New("notation_system must be FDI or UNIVERSAL")
	ErrInvalidDentalSurface    = errors.New("surface must be MESIAL, DISTAL, OCLUSAL_INCISAL, VESTIBULAR, LINGUAL_PALATINO, or WHOLE_TOOTH")
	ErrInvalidFindingType      = errors.New("finding_type is not in the standardized clinical catalog")
	ErrInvalidFindingStatus    = errors.New("status must be DIAGNOSED, PLANNED, or REALIZADO")
	ErrClinicalRecordImmutable = errors.New("RN-01: completed procedures/findings (REALIZADO) are strictly immutable; submit a signed addendum")
	ErrFindingNotFound         = errors.New("odontogram finding not found")
	ErrInvalidAddendum         = errors.New("addendum note requires a non-empty reason and recorded_by")
)

var fdiToUniversal = map[string]string{
	// Permanent upper right (18..11 -> 1..8)
	"18": "1", "17": "2", "16": "3", "15": "4", "14": "5", "13": "6", "12": "7", "11": "8",
	// Permanent upper left (21..28 -> 9..16)
	"21": "9", "22": "10", "23": "11", "24": "12", "25": "13", "26": "14", "27": "15", "28": "16",
	// Permanent lower left (38..31 -> 17..24)
	"38": "17", "37": "18", "36": "19", "35": "20", "34": "21", "33": "22", "32": "23", "31": "24",
	// Permanent lower right (41..48 -> 25..32)
	"41": "25", "42": "26", "43": "27", "44": "28", "45": "29", "46": "30", "47": "31", "48": "32",
	// Deciduous upper right (55..51 -> A..E)
	"55": "A", "54": "B", "53": "C", "52": "D", "51": "E",
	// Deciduous upper left (61..65 -> F..J)
	"61": "F", "62": "G", "63": "H", "64": "I", "65": "J",
	// Deciduous lower left (75..71 -> K..O)
	"75": "K", "74": "L", "73": "M", "72": "N", "71": "O",
	// Deciduous lower right (81..85 -> P..T)
	"81": "P", "82": "Q", "83": "R", "84": "S", "85": "T",
}

var universalToFDI = func() map[string]string {
	m := make(map[string]string, len(fdiToUniversal))
	for fdi, univ := range fdiToUniversal {
		m[univ] = fdi
	}
	return m
}()

var findingColorCatalog = map[string]string{
	"CARIES":               "#DC2626", // Red semantic alert per RF-03
	"EXISTING_RESTORATION": "#2563EB", // Blue semantic restoration per RF-03
	"ENDODONTICS_DONE":     "#2563EB",
	"ENDODONTICS_REQUIRED": "#DC2626",
	"IMPLANT":              "#0D9488",
	"FIXED_PROSTHESIS":     "#4F46E5",
	"REMOVABLE_PROSTHESIS": "#7C3AED",
	"EXTRACTION_DONE":      "#64748B",
	"EXTRACTION_INDICATED": "#DC2626",
	"SEALANT":              "#16A34A",
}

type FindingAddendum struct {
	ID         string    `json:"id"`
	FindingID  string    `json:"finding_id"`
	Note       string    `json:"note"`
	RecordedBy string    `json:"recorded_by"`
	RecordedAt time.Time `json:"recorded_at"`
}

type OdontogramFinding struct {
	ID             string            `json:"id"`
	PatientID      string            `json:"patient_id"`
	ToothFDI       string            `json:"tooth_fdi"`
	ToothUniversal string            `json:"tooth_universal"`
	DentitionType  string            `json:"dentition_type"`
	Surface        string            `json:"surface"`
	FindingType    string            `json:"finding_type"`
	ColorHex       string            `json:"color_hex"`
	Status         string            `json:"status"`
	Immutable      bool              `json:"immutable"`
	Notes          string            `json:"notes,omitempty"`
	RecordedBy     string            `json:"recorded_by"`
	RecordedAt     time.Time         `json:"recorded_at"`
	Addenda        []FindingAddendum `json:"addenda,omitempty"`
}

func ResolveToothCode(code, notation string) (fdi, universal, dentition string, err error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	notation = strings.ToUpper(strings.TrimSpace(notation))
	switch notation {
	case "FDI":
		univ, ok := fdiToUniversal[code]
		if !ok {
			return "", "", "", ErrInvalidToothCode
		}
		fdi = code
		universal = univ
	case "UNIVERSAL":
		mappedFDI, ok := universalToFDI[code]
		if !ok {
			return "", "", "", ErrInvalidToothCode
		}
		fdi = mappedFDI
		universal = code
	default:
		return "", "", "", ErrInvalidNotationSystem
	}

	if fdi[0] >= '1' && fdi[0] <= '4' {
		dentition = "PERMANENT"
	} else {
		dentition = "DECIDUOUS"
	}
	return fdi, universal, dentition, nil
}

func NewOdontogramFinding(id, patientID, toothCode, notation, surface, findingType, status, notes, recordedBy string, now time.Time) (*OdontogramFinding, error) {
	fdi, univ, dentition, err := ResolveToothCode(toothCode, notation)
	if err != nil {
		return nil, err
	}

	surface = strings.ToUpper(strings.TrimSpace(surface))
	switch surface {
	case "MESIAL", "DISTAL", "OCLUSAL_INCISAL", "VESTIBULAR", "LINGUAL_PALATINO", "WHOLE_TOOTH":
	default:
		return nil, ErrInvalidDentalSurface
	}

	findingType = strings.ToUpper(strings.TrimSpace(findingType))
	colorHex, ok := findingColorCatalog[findingType]
	if !ok {
		return nil, ErrInvalidFindingType
	}

	status = strings.ToUpper(strings.TrimSpace(status))
	switch status {
	case "DIAGNOSED", "PLANNED", "REALIZADO":
	default:
		return nil, ErrInvalidFindingStatus
	}

	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}

	return &OdontogramFinding{
		ID:             id,
		PatientID:      strings.TrimSpace(patientID),
		ToothFDI:       fdi,
		ToothUniversal: univ,
		DentitionType:  dentition,
		Surface:        surface,
		FindingType:    findingType,
		ColorHex:       colorHex,
		Status:         status,
		Immutable:      status == "REALIZADO",
		Notes:          strings.TrimSpace(notes),
		RecordedBy:     strings.TrimSpace(recordedBy),
		RecordedAt:     now,
		Addenda:        []FindingAddendum{},
	}, nil
}

func (f *OdontogramFinding) UpdateStatus(newStatus, notes string) error {
	if f.Status == "REALIZADO" || f.Immutable {
		return ErrClinicalRecordImmutable
	}
	newStatus = strings.ToUpper(strings.TrimSpace(newStatus))
	switch newStatus {
	case "DIAGNOSED", "PLANNED", "REALIZADO":
	default:
		return ErrInvalidFindingStatus
	}
	f.Status = newStatus
	f.Immutable = newStatus == "REALIZADO"
	if strings.TrimSpace(notes) != "" {
		f.Notes = strings.TrimSpace(notes)
	}
	return nil
}

func (f *OdontogramFinding) CanDelete() error {
	if f.Status == "REALIZADO" || f.Immutable {
		return ErrClinicalRecordImmutable
	}
	return nil
}

func (f *OdontogramFinding) AddAddendum(id, note, recordedBy string, now time.Time) (*FindingAddendum, error) {
	note = strings.TrimSpace(note)
	recordedBy = strings.TrimSpace(recordedBy)
	if note == "" || recordedBy == "" {
		return nil, ErrInvalidAddendum
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	add := FindingAddendum{
		ID:         id,
		FindingID:  f.ID,
		Note:       note,
		RecordedBy: recordedBy,
		RecordedAt: now,
	}
	f.Addenda = append(f.Addenda, add)
	return &add, nil
}
