package raft3d

import (
	"encoding/json"
	"github.com/hashicorp/raft"
	"io"
	"log"

)

// Apply applies a Raft log entry to the state machine.
func (rn *RaftNode) Apply(logEntry *raft.Log) interface{} {
	var cmd struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}

	if err := json.Unmarshal(logEntry.Data, &cmd); err != nil {
		log.Printf("Failed to unmarshal log: %v", err)
		return nil
	}

	switch cmd.Type {
	case "SET_PRINTER":
		var printer Printer
		if err := json.Unmarshal(cmd.Data, &printer); err == nil {
			rn.Printers[printer.ID] = printer
		}

	case "SET_FILAMENT":
		var filament Filament
		if err := json.Unmarshal(cmd.Data, &filament); err == nil {
			rn.Filaments[filament.ID] = filament
		}

	case "SET_PRINT_JOB":
		var job PrintJob
		if err := json.Unmarshal(cmd.Data, &job); err == nil {
			job.Status = "Queued" // Force status
	
			// Check filament existence
			filament, ok := rn.Filaments[job.FilamentID]
			if !ok {
				log.Printf("Invalid filament ID: %s", job.FilamentID)
				return map[string]string{"error": "Invalid filament ID: " + job.FilamentID}
			}
	
			// Sum up weights of queued/running jobs using this filament
			totalReserved := 0
			for _, j := range rn.PrintJobs {
				if j.FilamentID == job.FilamentID && (j.Status == "Queued" || j.Status == "Running") {
					totalReserved += j.PrintWeightInGrams
				}
			}
	
			available := filament.RemainingWeightInGrams - totalReserved
			if job.PrintWeightInGrams <= available {
				rn.PrintJobs[job.ID] = job
				return map[string]string{"message": "Print job created and queued successfully"}
			} else {
				log.Printf("Not enough filament weight for job %s: needed %d, available %d", job.ID, job.PrintWeightInGrams, available)
				return map[string]string{"error": "Not enough filament available"}
			}
		}
	
	
	
	case "UPDATE_PRINT_JOB_STATUS":
		var payload map[string]string
		if err := json.Unmarshal(cmd.Data, &payload); err == nil {
			jobID := payload["job_id"]
			newStatus := payload["status"]
	
			job, exists := rn.PrintJobs[jobID]
			if !exists {
				log.Printf("Job ID not found: %s", jobID)
				return map[string]string{"error": "Job ID not found"}
			}
	
			valid := false
			switch newStatus {
			case "Running":
				if job.Status == "Queued" {
					valid = true
				}
			case "Done":
				if job.Status == "Running" {
					valid = true
					// Deduct weight
					filament := rn.Filaments[job.FilamentID]
					filament.RemainingWeightInGrams -= job.PrintWeightInGrams
					rn.Filaments[job.FilamentID] = filament
				}
			case "Canceled":
				if job.Status == "Queued" || job.Status == "Running" {
					valid = true
				}
			}
	
			if valid {
				job.Status = newStatus
				rn.PrintJobs[jobID] = job
				return map[string]string{"message": "Status updated successfully"}
			} else {
				log.Printf("Invalid status transition: %s -> %s", job.Status, newStatus)
				return map[string]string{"error": "Invalid status transition: " + job.Status + " -> " + newStatus}
			}
		}
	

	case "SET_KEY_VALUE":
		var kv map[string]string
		if err := json.Unmarshal(cmd.Data, &kv); err == nil {
			for k, v := range kv {
				rn.store[k] = v
			}
		}
	}
	return nil
}

// Snapshot captures the current state of the FSM.
func (rn *RaftNode) Snapshot() (raft.FSMSnapshot, error) {
	data := snapshotData{
		Printers:  rn.Printers,
		Filaments: rn.Filaments,
		PrintJobs: rn.PrintJobs,
		Store:     rn.store,
	}
	return &snapshot{data: data}, nil
}

// Restore restores the FSM from a snapshot.
func (rn *RaftNode) Restore(reader io.ReadCloser) error {
	defer reader.Close()

	var data snapshotData
	if err := json.NewDecoder(reader).Decode(&data); err != nil {
		return err
	}

	rn.Printers = data.Printers
	rn.Filaments = data.Filaments
	rn.PrintJobs = data.PrintJobs
	rn.store = data.Store
	return nil
}

// Struct for storing the entire FSM snapshot data
type snapshotData struct {
	Printers  map[string]Printer  `json:"printers"`
	Filaments map[string]Filament `json:"filaments"`
	PrintJobs map[string]PrintJob `json:"print_jobs"`
	Store     map[string]string   `json:"store"`
}

// snapshot implements raft.FSMSnapshot
type snapshot struct {
	data snapshotData
}

func (s *snapshot) Persist(sink raft.SnapshotSink) error {
	data, err := json.Marshal(s.data)
	if err != nil {
		sink.Cancel()
		return err
	}
	if _, err := sink.Write(data); err != nil {
		sink.Cancel()
		return err
	}
	return sink.Close()
}

func (s *snapshot) Release() {}
