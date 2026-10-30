package domain

import (
	"errors"
	"math"
	"strings"
	"time"
)

var (
	ErrInvalidPerioChart      = errors.New("periodontogram requires at least one evaluated tooth")
	ErrInvalidProbingDepth    = errors.New("probing_depth_mm must contain 6 values in range 0..15")
	ErrInvalidGingivalMargin  = errors.New("gingival_margin_mm must contain 6 values in range -10..10")
	ErrInvalidFurcationGrade  = errors.New("furcation_grade must be between 0 and 3")
	ErrInvalidMobilityGrade   = errors.New("mobility_grade must be between 0 and 3")
)

type PeriodontogramTooth struct {
	ToothFDI          string  `json:"tooth_fdi"`
	ProbingDepthMM    [6]int  `json:"probing_depth_mm"`
	GingivalMarginMM  [6]int  `json:"gingival_margin_mm"`
	NICMM             [6]int  `json:"nic_mm"`
	BleedingOnProbing [6]bool `json:"bleeding_on_probing"`
	FurcationGrade    int     `json:"furcation_grade"`
	MobilityGrade     int     `json:"mobility_grade"`
}

type PeriodontogramChart struct {
	ID            string                `json:"id"`
	PatientID     string                `json:"patient_id"`
	BOPPercentage float64               `json:"bop_percentage"`
	MeanNICMM     float64               `json:"mean_nic_mm"`
	Teeth         []PeriodontogramTooth `json:"teeth"`
	RecordedBy    string                `json:"recorded_by"`
	RecordedAt    time.Time             `json:"recorded_at"`
}

func NewPeriodontogramChart(id, patientID, recordedBy string, teethInput []PeriodontogramTooth, now time.Time) (*PeriodontogramChart, error) {
	if strings.TrimSpace(patientID) == "" || len(teethInput) == 0 {
		return nil, ErrInvalidPerioChart
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}

	evaluated := make([]PeriodontogramTooth, 0, len(teethInput))
	totalSites := 0
	bleedingSites := 0
	sumNIC := 0

	for _, t := range teethInput {
		fdi, _, _, err := ResolveToothCode(t.ToothFDI, "FDI")
		if err != nil {
			return nil, err
		}
		if t.FurcationGrade < 0 || t.FurcationGrade > 3 {
			return nil, ErrInvalidFurcationGrade
		}
		if t.MobilityGrade < 0 || t.MobilityGrade > 3 {
			return nil, ErrInvalidMobilityGrade
		}

		var nic [6]int
		for i := 0; i < 6; i++ {
			pd := t.ProbingDepthMM[i]
			gm := t.GingivalMarginMM[i]
			if pd < 0 || pd > 15 {
				return nil, ErrInvalidProbingDepth
			}
			if gm < -10 || gm > 10 {
				return nil, ErrInvalidGingivalMargin
			}
			// NIC = PD - GM per RF-04 / S4
			nic[i] = pd - gm
			sumNIC += nic[i]
			totalSites++
			if t.BleedingOnProbing[i] {
				bleedingSites++
			}
		}

		evaluated = append(evaluated, PeriodontogramTooth{
			ToothFDI:          fdi,
			ProbingDepthMM:    t.ProbingDepthMM,
			GingivalMarginMM:  t.GingivalMarginMM,
			NICMM:             nic,
			BleedingOnProbing: t.BleedingOnProbing,
			FurcationGrade:    t.FurcationGrade,
			MobilityGrade:     t.MobilityGrade,
		})
	}

	bopPct := math.Round((float64(bleedingSites)/float64(totalSites))*10000) / 100
	meanNIC := math.Round((float64(sumNIC)/float64(totalSites))*100) / 100

	return &PeriodontogramChart{
		ID:            id,
		PatientID:     strings.TrimSpace(patientID),
		BOPPercentage: bopPct,
		MeanNICMM:     meanNIC,
		Teeth:         evaluated,
		RecordedBy:    strings.TrimSpace(recordedBy),
		RecordedAt:    now,
	}, nil
}
