package raft3d

// Printer represents a 3D printer with unique ID, manufacturer details, and model.
type Printer struct {
	ID      string `json:"id"`      // Unique identifier (string or int as string)
	Company string `json:"company"` // e.g., Creality, Prusa
	Model   string `json:"model"`   // e.g., Ender 3, i3 MK3S+
}

// Filament represents a spool of plastic filament used in 3D printing.
type Filament struct {
	ID                      string `json:"id"`                        // Unique identifier (string or int as string)
	Type                    string `json:"type"`                      // PLA, PETG, ABS, TPU
	Color                   string `json:"color"`                     // e.g., red, blue, black
	TotalWeightInGrams      int    `json:"total_weight_in_grams"`    // e.g., 1000 for 1kg roll
	RemainingWeightInGrams  int    `json:"remaining_weight_in_grams"`// updated after each print
}

// PrintJob represents a job to print a specific item using a printer and filament.
type PrintJob struct {
	ID                 string `json:"id"`                  // Unique job ID (string or int as string)
	PrinterID          string `json:"printer_id"`          // Must match a valid Printer.ID
	FilamentID         string `json:"filament_id"`         // Must match a valid Filament.ID
	Filepath           string `json:"filepath"`            // e.g., prints/sword/hilt.gcode
	PrintWeightInGrams int    `json:"print_weight_in_grams"` // deducted from filament
	Status             string `json:"status"`              // Queued, Running, Cancelled, Done
}
