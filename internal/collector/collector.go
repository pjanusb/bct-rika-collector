package collector

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"rika-collector/internal/config"
	"rika-collector/internal/sshclient"
)

type collector struct {
	ssh           *sshclient.Client
	executableDir string
	outputDir     string
}

type section struct {
	name string
	run  func() error
}

func Run(configPath string) int {
	cfg, err := config.Load(configPath)
	if err != nil {
		errorLog("%v", err)
		return 1
	}

	executablePath, err := os.Executable()
	if err != nil {
		errorLog("Cannot determine executable path: %v", err)
		return 1
	}
	executableDir, err := filepath.Abs(filepath.Dir(executablePath))
	if err != nil {
		errorLog("Cannot determine executable directory: %v", err)
		return 1
	}

	timestamp := time.Now().Format("20060102_150405")
	outputBaseName := "rika_files_" + timestamp
	artifactsDir := filepath.Join(executableDir, "collected_files")
	outputDir := filepath.Join(artifactsDir, outputBaseName)
	archivePath := filepath.Join(artifactsDir, outputBaseName+".tgz")
	summaryPath := filepath.Join(outputDir, "collection_summary.txt")

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		errorLog("Cannot create output directory: %v", err)
		return 1
	}

	target := cfg.User + "@" + cfg.IP
	if err := writeSummaryHeader(summaryPath, target, outputDir); err != nil {
		errorLog("Cannot create summary file: %v", err)
		os.RemoveAll(outputDir)
		return 1
	}

	log("Checking SSH connection to %s", target)
	sshConnection, err := sshclient.Dial(cfg.User, cfg.Password, net.JoinHostPort(cfg.IP, "22"))
	if err != nil {
		errorLog("Cannot connect to %s: %v", target, err)
		os.RemoveAll(outputDir)
		return 1
	}
	defer sshConnection.Close()

	if _, err := sshConnection.CombinedOutput("true", nil); err != nil {
		errorLog("Cannot connect to %s: %v", target, err)
		os.RemoveAll(outputDir)
		return 1
	}

	instance := &collector{
		ssh:           sshConnection,
		executableDir: executableDir,
		outputDir:     outputDir,
	}

	sections := []section{
		{name: "sub_001_redis_network_info.sh", run: instance.collectRedisNetworkInfo},
		{name: "sub_002_data_profiles.sh", run: instance.collectDataProfiles},
		{name: "sub_003_dlogs.sh", run: instance.collectDlogs},
		{name: "sub_004_system_commands.sh", run: instance.collectSystemCommands},
		{name: "sub_005_networkmanager_connections.sh", run: instance.collectNetworkManagerConnections},
		{name: "sub_006_networkmanager_dump.sh", run: instance.collectNetworkManagerDump},
		{name: "sub_007_etc_dehydrated.sh", run: instance.collectEtcDehydrated},
		{name: "sub_008_data_dehydrated.sh", run: instance.collectDataDehydrated},
		{name: "sub_009_tmp_dehydrated.sh", run: instance.collectTmpDehydrated},
	}

	failed := false
	for _, current := range sections {
		log("Running %s", current.name)
		if err := current.run(); err != nil {
			line := fmt.Sprintf("[FAILED] %s (exit code: %d)\n", current.name, failureExitCode(err))
			fmt.Print(line)
			appendSummary(summaryPath, line)
			failed = true
			continue
		}

		line := fmt.Sprintf("[OK]     %s\n", current.name)
		fmt.Print(line)
		appendSummary(summaryPath, line)
	}

	appendSummary(summaryPath, "\nCollection finished: "+time.Now().Format(time.RFC3339)+"\n")

	log("Creating archive: %s", archivePath)
	if err := createArchive(outputDir, archivePath); err != nil {
		errorLog("Failed to create archive: %v", err)
		return 1
	}
	log("Archive created: %s", archivePath)

	if failed {
		warn("One or more sections failed. See %s.", summaryPath)
		return 2
	}
	return 0
}

func failureExitCode(err error) int {
	var remoteStatus interface {
		ExitStatus() int
	}
	if errors.As(err, &remoteStatus) {
		return remoteStatus.ExitStatus()
	}

	var localStatus interface {
		ExitCode() int
	}
	if errors.As(err, &localStatus) {
		return localStatus.ExitCode()
	}
	return 1
}

func writeSummaryHeader(path string, target string, outputDir string) error {
	content := fmt.Sprintf("Collection started: %s\nTarget:             %s\nOutput directory:   %s\n\n", time.Now().Format(time.RFC3339), target, outputDir)
	return os.WriteFile(path, []byte(content), 0o644)
}

func appendSummary(path string, content string) {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		warn("Cannot update summary file %s: %v", path, err)
		return
	}
	defer file.Close()
	if _, err := file.WriteString(content); err != nil {
		warn("Cannot update summary file %s: %v", path, err)
	}
}

func log(format string, args ...any) {
	fmt.Printf("[collector] "+format+"\n", args...)
}

func warn(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[collector] WARNING: "+format+"\n", args...)
}

func errorLog(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[collector] ERROR: "+format+"\n", args...)
}
