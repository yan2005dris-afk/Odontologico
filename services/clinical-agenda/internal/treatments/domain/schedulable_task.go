package domain

type SchedulableTask struct {
	TaskID                   string `json:"task_id"`
	PatientID                string `json:"patient_id"`
	PlanID                   string `json:"plan_id"`
	TreatmentItemID          string `json:"treatment_item_id"`
	PhaseNumber              int    `json:"phase_number"`
	ProcedureCode            string `json:"procedure_code"`
	ProcedureName            string `json:"procedure_name"`
	ToothFDI                 string `json:"tooth_fdi,omitempty"`
	EstimatedDurationMinutes int    `json:"estimated_duration_minutes"`
	Status                   string `json:"status"` // READY_TO_SCHEDULE | SCHEDULED | COMPLETED
}
