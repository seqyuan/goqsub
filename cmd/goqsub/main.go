package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/akamensky/argparse"
	"github.com/dgruber/drmaa"
)

func main() {
	parser := argparse.NewParser("goqsub", "Submit a single task to qsub SGE system")
	opt_i := parser.String("i", "i", &argparse.Options{Required: true, Help: "Input shell script file"})
	opt_cpu := parser.Int("", "cpu", &argparse.Options{Default: 1, Help: "Number of CPUs per task (default: 1)"})
	opt_mem := parser.Int("", "mem", &argparse.Options{Required: false, Help: "Memory in GB per task (only used if explicitly set)"})
	opt_h_vmem := parser.Int("", "h_vmem", &argparse.Options{Required: false, Help: "Virtual memory in GB per task (only used if explicitly set)"})
	opt_queue := parser.String("", "queue", &argparse.Options{Default: "scv.q,sci.q", Help: "Queue name(s), comma-separated for multiple queues (default: scv.q,sci.q)"})
	opt_sge_project := parser.String("P", "sge-project", &argparse.Options{Required: false, Help: "SGE project name for resource quota management (optional)"})

	// Check if user explicitly set --mem or --h_vmem before parsing
	userSetMem := false
	userSetHvmem := false
	for _, arg := range os.Args[1:] {
		if arg == "--mem" {
			userSetMem = true
		}
		if arg == "--h_vmem" {
			userSetHvmem = true
		}
	}

	err := parser.Parse(os.Args)
	if err != nil {
		fmt.Print(parser.Usage(err))
		os.Exit(1)
	}

	// Get values
	scriptPath := *opt_i
	cpu := *opt_cpu
	mem := *opt_mem
	h_vmem := *opt_h_vmem
	queue := ""
	if opt_queue != nil {
		rawQueue := *opt_queue
		if rawQueue != "" {
			// Clean up queue string: trim whitespace and trailing commas
			queue = strings.TrimSpace(rawQueue)
			// Remove all trailing commas (multiple passes to be sure)
			for strings.HasSuffix(queue, ",") {
				queue = strings.TrimSuffix(queue, ",")
			}
			queue = strings.TrimRight(queue, " \t")
			// Remove spaces around commas
			queue = strings.ReplaceAll(queue, ", ", ",")
			queue = strings.ReplaceAll(queue, " ,", ",")
		}
	}
	sgeProject := ""
	if opt_sge_project != nil {
		sgeProject = strings.TrimSpace(*opt_sge_project)
	}

	// Validate script file exists
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		log.Fatalf("Error: Script file does not exist: %s", scriptPath)
	}

	// Get absolute path and directory
	absScriptPath, err := filepath.Abs(scriptPath)
	if err != nil {
		log.Fatalf("Error: Could not get absolute path for script: %v", err)
	}
	absScriptDir := filepath.Dir(absScriptPath)

	// #region agent log
	// Debug: Log script path and directory
	debugLog("main.go:77", "Script path resolution", map[string]interface{}{
		"scriptPath":    scriptPath,
		"absScriptPath": absScriptPath,
		"absScriptDir":  absScriptDir,
		"hypothesisId":  "A",
	})
	// #endregion

	// Submit job
	jobID, err := submitJob(absScriptPath, absScriptDir, cpu, mem, h_vmem, userSetMem, userSetHvmem, queue, sgeProject)
	if err != nil {
		log.Fatalf("Error submitting job: %v", err)
	}

	fmt.Printf("%s\n", jobID)
}

// submitJob submits a single job to qsub SGE system using DRMAA
func submitJob(scriptPath, scriptDir string, cpu, mem, h_vmem int, userSetMem, userSetHvmem bool, queue, sgeProject string) (string, error) {
	// Save current working directory and switch to script directory
	// This ensures that -cwd will set SGE's working directory to script directory
	originalDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %v", err)
	}

	// Switch to script directory before submitting job
	err = os.Chdir(scriptDir)
	if err != nil {
		return "", fmt.Errorf("failed to change to script directory %s: %v", scriptDir, err)
	}
	// Restore original directory after job submission
	defer os.Chdir(originalDir)

	// #region agent log
	// Debug: Log directory change
	debugLog("main.go:101", "Directory change for job submission", map[string]interface{}{
		"originalDir":  originalDir,
		"scriptDir":    scriptDir,
		"hypothesisId": "F",
	})
	// #endregion

	// Create DRMAA session
	session, err := drmaa.MakeSession()
	if err != nil {
		return "", fmt.Errorf("failed to create DRMAA session: %v", err)
	}
	defer session.Exit()

	// Create job template
	jt, err := session.AllocateJobTemplate()
	if err != nil {
		return "", fmt.Errorf("failed to allocate job template: %v", err)
	}
	defer session.DeleteJobTemplate(&jt)

	// Get base name of script
	scriptBase := filepath.Base(scriptPath)

	// #region agent log
	// Debug: Log script base name and current working directory
	wd, _ := os.Getwd()
	debugLog("main.go:105", "Script base name and working directory", map[string]interface{}{
		"scriptBase":   scriptBase,
		"scriptPath":   scriptPath,
		"scriptDir":    scriptDir,
		"currentDir":   wd,
		"hypothesisId": "B",
	})
	// #endregion

	// Set job template properties
	// Use absolute script path to ensure SGE can find the script
	jt.SetRemoteCommand(scriptPath)
	// Set job name to script base name (with extension)
	// SGE will automatically create output files with default naming: job-name.ojob-id and job-name.ejob-id
	jt.SetJobName(scriptBase)

	// Build nativeSpec with SGE resource options
	// Use -cwd to set working directory to current directory (which is now script directory)
	// This ensures output files are generated in the script's directory

	// Clean queue string first
	queueClean := ""
	if queue != "" {
		// First remove all spaces
		queueClean = strings.ReplaceAll(queue, " ", "")
		// Then trim trailing commas and tabs
		queueClean = strings.TrimRight(queueClean, ", \t")
		// Remove spaces around commas (in case there are any left)
		queueClean = strings.ReplaceAll(queueClean, ", ", ",")
		queueClean = strings.ReplaceAll(queueClean, " ,", ",")
		// Final trim
		queueClean = strings.TrimSpace(queueClean)
		queueClean = strings.TrimRight(queueClean, ", \t")
	}

	// Build nativeSpec matching: qsub -pe smp 4 -l h_vmem=10g -cwd -b n L2_1_1.sh
	// Use -cwd to set working directory to current directory (which is now script directory)
	// SGE will automatically create output files in the working directory with default naming:
	// job-name.ojob-id and job-name.ejob-id (where job-name is set via SetJobName)
	// This ensures output files are generated in the script's directory
	// Start with parallel environment, -cwd, and -b n (non-binary mode, use shell)
	nativeSpec := fmt.Sprintf("-pe smp %d -cwd -b n", cpu)

	// #region agent log
	// Debug: Log initial nativeSpec with -cwd
	currentDir, _ := os.Getwd()
	debugLog("main.go:187", "Initial nativeSpec with -cwd", map[string]interface{}{
		"nativeSpec":   nativeSpec,
		"scriptDir":    scriptDir,
		"scriptBase":   scriptBase,
		"currentDir":   currentDir,
		"hypothesisId": "C",
	})
	// #endregion

	// Add queue if provided
	if queueClean != "" {
		nativeSpec += fmt.Sprintf(" -q %s", queueClean)
	}

	// Build resource list: SGE supports comma-separated resources in a single -l option
	// Format: -l vf=8g,h_vmem=8g
	// Use lowercase "g" for GB unit
	var resources []string

	if userSetMem {
		// SGE uses "vf" (virtual free memory) instead of "mem"
		resources = append(resources, fmt.Sprintf("vf=%dg", mem))
	}
	if userSetHvmem {
		resources = append(resources, fmt.Sprintf("h_vmem=%dg", h_vmem))
	}

	// Combine all resources into a single -l option
	if len(resources) > 0 {
		resourceSpec := strings.Join(resources, ",")
		nativeSpec += fmt.Sprintf(" -l %s", resourceSpec)
	}
	// Add SGE project specification if provided (for resource quota management)
	if sgeProject != "" {
		nativeSpec += fmt.Sprintf(" -P %s", sgeProject)
	}

	// #region agent log
	// Debug: Log final nativeSpec before setting
	debugLog("main.go:167", "Final nativeSpec before SetNativeSpecification", map[string]interface{}{
		"nativeSpec":   nativeSpec,
		"scriptDir":    scriptDir,
		"scriptBase":   scriptBase,
		"hypothesisId": "D",
	})
	// #endregion

	jt.SetNativeSpecification(nativeSpec)

	// Submit job
	jobID, err := session.RunJob(&jt)

	// #region agent log
	// Debug: Log job submission result
	debugLog("main.go:170", "Job submission result", map[string]interface{}{
		"jobID":        jobID,
		"error":        fmt.Sprintf("%v", err),
		"scriptDir":    scriptDir,
		"scriptBase":   scriptBase,
		"hypothesisId": "E",
	})
	// #endregion

	if err != nil {
		// Provide more detailed error information
		errMsg := fmt.Sprintf("failed to submit job: %v", err)
		if queueClean != "" {
			errMsg += fmt.Sprintf("\nQueue specified: %s", queueClean)
			errMsg += "\nTroubleshooting tips:"
			errMsg += "\n  1. Check if queue exists: qconf -sql"
			errMsg += "\n  2. Check queue status: qstat -g c"
			errMsg += "\n  3. Check queue configuration: qconf -sq " + queueClean
			errMsg += "\n  4. Try without --queue parameter to use default queue"
		}
		return "", fmt.Errorf(errMsg)
	}

	return jobID, nil
}

// debugLog writes a debug log entry to the log file
func debugLog(location, message string, data map[string]interface{}) {
	logPath := "/Volumes/data/github/seqyuan/goqsub/.cursor/debug.log"
	logEntry := map[string]interface{}{
		"timestamp": time.Now().UnixMilli(),
		"location":  location,
		"message":   message,
		"sessionId": "debug-session",
		"runId":     "run1",
		"data":      data,
	}
	if hypothesisId, ok := data["hypothesisId"].(string); ok {
		logEntry["hypothesisId"] = hypothesisId
	}
	logJSON, _ := json.Marshal(logEntry)
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(string(logJSON) + "\n")
}
