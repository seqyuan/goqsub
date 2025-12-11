package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

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
		queue = *opt_queue
	}
	sgeProject := ""
	if opt_sge_project != nil {
		sgeProject = *opt_sge_project
	}

	// Validate script file exists
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		log.Fatalf("Error: Script file does not exist: %s", scriptPath)
	}

	// Get absolute path
	absScriptPath, err := filepath.Abs(scriptPath)
	if err != nil {
		log.Fatalf("Error: Could not get absolute path for script: %v", err)
	}

	// Submit job
	jobID, err := submitJob(absScriptPath, cpu, mem, h_vmem, userSetMem, userSetHvmem, queue, sgeProject)
	if err != nil {
		log.Fatalf("Error submitting job: %v", err)
	}

	fmt.Printf("Job submitted successfully. Job ID: %s\n", jobID)
}

// submitJob submits a single job to qsub SGE system using DRMAA
func submitJob(scriptPath string, cpu, mem, h_vmem int, userSetMem, userSetHvmem bool, queue, sgeProject string) (string, error) {
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

	// Get directory and base name of script
	scriptDir := filepath.Dir(scriptPath)
	scriptBase := filepath.Base(scriptPath)
	scriptBaseNoExt := strings.TrimSuffix(scriptBase, filepath.Ext(scriptBase))

	// Set job template properties
	// SetRemoteCommand sets the script path, which SGE will use as the command to execute
	// The working directory will be automatically set to the script's directory by SGE
	jt.SetRemoteCommand(scriptPath)
	// Set job name to file prefix, so SGE will auto-generate output files as:
	// {scriptBaseNoExt}.o.{jobID} and {scriptBaseNoExt}.e.{jobID}
	jt.SetJobName(scriptBaseNoExt)

	// Build nativeSpec with only SGE resource options
	// Do NOT include -cwd or script path in nativeSpec
	// - SetRemoteCommand already sets the script path, which SGE uses to determine working directory
	// - SGE automatically uses the script's directory as the working directory
	// - Output files will be generated in the script's directory: {job_name}.o.{jobID} and {job_name}.e.{jobID}
	// - Including -cwd in nativeSpec may cause parsing errors with some DRMAA implementations
	nativeSpec := fmt.Sprintf("-l cpu=%d", cpu)
	if userSetMem {
		nativeSpec += fmt.Sprintf(" -l mem=%dG", mem)
	}
	if userSetHvmem {
		nativeSpec += fmt.Sprintf(" -l h_vmem=%dG", h_vmem)
	}
	// Add queue specification if provided (supports multiple queues, comma-separated)
	if queue != "" {
		// Trim any trailing commas or whitespace from queue string
		queue = strings.TrimRight(strings.TrimSpace(queue), ", \t")
		if queue != "" {
			nativeSpec += fmt.Sprintf(" -q %s", queue)
		}
	}
	// Add SGE project specification if provided (for resource quota management)
	if sgeProject != "" {
		nativeSpec += fmt.Sprintf(" -P %s", sgeProject)
	}
	jt.SetNativeSpecification(nativeSpec)

	// Debug: log nativeSpec for troubleshooting
	log.Printf("DEBUG: nativeSpec: %s, scriptPath: %s, scriptDir: %s", nativeSpec, scriptPath, scriptDir)

	// Submit job
	jobID, err := session.RunJob(&jt)
	if err != nil {
		return "", fmt.Errorf("failed to submit job: %v", err)
	}

	return jobID, nil
}

