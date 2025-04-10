package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
	"github.com/hashicorp/raft"
	"cc_proj/raft3d"
)

var node *raft3d.RaftNode

func main() {
	nodeID := os.Getenv("NODE_ID")
	if nodeID == "" {
		nodeID = "node1"
	}

	raftDir := fmt.Sprintf("raft_data/%s", nodeID)

	raftAddr := os.Getenv("RAFT_ADDR")
	if raftAddr == "" {
		raftAddr = "127.0.0.1:9001"
	}

	httpAddr := os.Getenv("HTTP_ADDR")
	if httpAddr == "" {
		httpAddr = "127.0.0.1:8000"
	}

	node = raft3d.NewRaftNode(nodeID, raftDir, raftAddr)

	// Start leader logging goroutine
	go func() {
		for {
			time.Sleep(2 * time.Second)
			if node.Raft.State() == raft.Leader {
				log.Printf("[%s] I am the leader!", nodeID)
			}
		}
	}()

	http.HandleFunc("/join", joinHandler)
	http.HandleFunc("/set_printer", setPrinterHandler)
	http.HandleFunc("/get_printer", getPrinterHandler)
	http.HandleFunc("/set_filament", setFilamentHandler)
	http.HandleFunc("/get_filament", getFilamentHandler)
	http.HandleFunc("/set_print_job", setPrintJobHandler)
	http.HandleFunc("/get_print_job", getPrintJobHandler)
	http.HandleFunc("/status", statusHandler)
	http.HandleFunc("/update_status", updatePrintJobStatusHandler)

	log.Printf("Starting Raft3D HTTP server on %s...", httpAddr)
	log.Fatal(http.ListenAndServe(httpAddr, nil))
}


func joinHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		NodeID string `json:"node_id"`
		Addr   string `json:"addr"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	configFuture := node.Raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		http.Error(w, "Failed to get configuration", http.StatusInternalServerError)
		return
	}

	for _, srv := range configFuture.Configuration().Servers {
		if srv.ID == raft.ServerID(req.NodeID) || srv.Address == raft.ServerAddress(req.Addr) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Node already exists"))
			return
		}
	}

	f := node.Raft.AddVoter(raft.ServerID(req.NodeID), raft.ServerAddress(req.Addr), 0, 0)
	if err := f.Error(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to add node: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Node added successfully"))
}

func setPrinterHandler(w http.ResponseWriter, r *http.Request) {
	var printer raft3d.Printer
	if err := json.NewDecoder(r.Body).Decode(&printer); err != nil {
		http.Error(w, "Invalid printer data", http.StatusBadRequest)
		return
	}
	applyRaftCommand("SET_PRINTER", printer, w)
}

func getPrinterHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing printer ID", http.StatusBadRequest)
		return
	}

	printer, exists := node.Printers[id]
	if !exists {
		http.Error(w, "Printer not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(printer)
}


func setFilamentHandler(w http.ResponseWriter, r *http.Request) {
	var filament raft3d.Filament
	if err := json.NewDecoder(r.Body).Decode(&filament); err != nil {
		http.Error(w, "Invalid filament data", http.StatusBadRequest)
		return
	}
	applyRaftCommand("SET_FILAMENT", filament, w)
}

func getFilamentHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing filament ID", http.StatusBadRequest)
		return
	}

	filament, exists := node.Filaments[id]
	if !exists {
		http.Error(w, "Filament not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filament)
}


func setPrintJobHandler(w http.ResponseWriter, r *http.Request) {
	var job raft3d.PrintJob
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		http.Error(w, "Invalid print job data", http.StatusBadRequest)
		return
	}
	applyRaftCommand("SET_PRINT_JOB", job, w)
}

func getPrintJobHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing print job ID", http.StatusBadRequest)
		return
	}

	job, exists := node.PrintJobs[id]
	if !exists {
		http.Error(w, "Print job not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}



func statusHandler(w http.ResponseWriter, r *http.Request) {
	resp := map[string]interface{}{
		"printers":  node.Printers,
		"filaments": node.Filaments,
		"printJobs": node.PrintJobs,
		"leader":    node.Raft.Leader(),
		"state":     node.Raft.State().String(),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func applyRaftCommand(cmdType string, data interface{}, w http.ResponseWriter) {
	if node.Raft.State() != raft.Leader {
		http.Error(w, "This node is not the leader", http.StatusForbidden)
		return
	}

	payload, err := json.Marshal(map[string]interface{}{
		"type": cmdType,
		"data": data,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to encode command: %v", err), http.StatusInternalServerError)
		return
	}

	applyFuture := node.Raft.Apply(payload, 5*time.Second)
	if err := applyFuture.Error(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to apply command: %v", err), http.StatusInternalServerError)
		return
	}

	resp := applyFuture.Response()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}


func updatePrintJobStatusHandler(w http.ResponseWriter, r *http.Request) {
	jobID := r.URL.Query().Get("job_id")
	newStatus := r.URL.Query().Get("status")

	if jobID == "" || newStatus == "" {
		http.Error(w, "Missing job_id or status", http.StatusBadRequest)
		return
	}

	data := map[string]string{
		"job_id": jobID,
		"status": newStatus,
	}
	applyRaftCommand("UPDATE_PRINT_JOB_STATUS", data, w)
}


