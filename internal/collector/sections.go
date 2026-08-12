package collector

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

func (c *collector) collectRedisNetworkInfo() error {
	outputPath := filepath.Join(c.outputDir, "redis_network_info.txt")
	log("Reading NetworkInfoData from Redis")
	command := `
SOCK="/data/share/redis.sock"
KEY="redis-shared-data:NetworkInfoData:NetworkInfo"

if ! command -v redis-cli >/dev/null 2>&1; then
    echo "redis-cli is not available" >&2
    exit 127
fi

redis-cli -s "$SOCK" --raw GET "$KEY"
`
	return c.captureRemoteCommand(outputPath, command)
}

func (c *collector) collectDataProfiles() error {
	log("Copying complete /data/profiles tree")
	return c.copyRemoteTree("/data/profiles", filepath.Join(c.outputDir, "data", "profiles"))
}

func (c *collector) collectDlogs() error {
	dlogDir := filepath.Join(c.outputDir, "data", "log")
	if err := os.MkdirAll(dlogDir, 0o755); err != nil {
		return err
	}

	allDlogFiles, err := c.remoteDlogFiles()
	if err != nil {
		return fmt.Errorf("failed to list .dlog files: %w", err)
	}

	allDlogFilesPath := filepath.Join(dlogDir, "_all_dlog_files.txt")
	allDlogFilesContent := strings.Join(allDlogFiles, "\n")
	if len(allDlogFiles) > 0 {
		allDlogFilesContent += "\n"
	}
	if err := os.WriteFile(allDlogFilesPath, []byte(allDlogFilesContent), 0o644); err != nil {
		return err
	}

	if len(allDlogFiles) == 0 {
		warn("No .dlog files found in /data/log")
		return nil
	}

	currentDlogFile, err := c.currentDlogFile()
	if err != nil {
		return fmt.Errorf("failed to resolve /data/log/currentDlogLink: %w", err)
	}

	selectedDlogFiles, err := selectDlogFiles(allDlogFiles, currentDlogFile, c.dlogDays)
	if err != nil {
		return err
	}

	log("Dlog files available: %d, selected: %d", len(allDlogFiles), len(selectedDlogFiles))
	command := "cd /data/log || exit 1\ntar -cf -"
	for _, name := range selectedDlogFiles {
		command += " " + shellQuote("./"+name)
	}

	if err := c.copyRemoteTar(command, dlogDir); err != nil {
		return fmt.Errorf("failed to copy .dlog files: %w", err)
	}

	if runtime.GOOS != "linux" {
		log("Skipping dlog conversion on %s", runtime.GOOS)
		return nil
	}

	parserPath := filepath.Join(c.executableDir, "dlogparser")
	parserInfo, err := os.Stat(parserPath)
	if err != nil || parserInfo.IsDir() || parserInfo.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("dlog parser is missing or not executable: %s", parserPath)
	}

	entries, err := os.ReadDir(dlogDir)
	if err != nil {
		return err
	}

	var dlogFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".dlog" {
			dlogFiles = append(dlogFiles, filepath.Join(dlogDir, entry.Name()))
		}
	}
	sort.Strings(dlogFiles)
	log("Dlog files to parse: %d", len(dlogFiles))

	failed := false
	for _, dlogFile := range dlogFiles {
		log("Parsing %s", filepath.Base(dlogFile))
		command := exec.Command(parserPath, dlogFile, dlogDir)
		command.Dir = dlogDir
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		if err := command.Run(); err != nil {
			errorLog("Failed to parse %s", filepath.Base(dlogFile))
			failed = true
			continue
		}
		if err := os.Remove(dlogFile); err != nil {
			errorLog("Failed to remove parsed file %s: %v", filepath.Base(dlogFile), err)
			failed = true
		}
	}

	if failed {
		return fmt.Errorf("one or more dlog files could not be converted")
	}
	return nil
}

func (c *collector) remoteDlogFiles() ([]string, error) {
	output, err := c.ssh.CombinedOutput(`cd /data/log || exit 1
for file in ./*.dlog; do
    [ -f "$file" ] || continue
    printf '%s\n' "${file#./}"
done`, nil)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, line := range strings.Split(string(output), "\n") {
		name := strings.TrimSpace(line)
		if name != "" {
			files = append(files, name)
		}
	}
	sort.Strings(files)
	return files, nil
}

func (c *collector) currentDlogFile() (string, error) {
	output, err := c.ssh.CombinedOutput(`cd /data/log || exit 1
target="$(readlink currentDlogLink)" || exit 1
[ -n "$target" ] || exit 1
case "$target" in
    /data/log/*) target="${target#/data/log/}" ;;
    ./*) target="${target#./}" ;;
esac
case "$target" in
    */*) exit 1 ;;
esac
[ -f "$target" ] || exit 1
printf '%s\n' "$target"`, nil)
	if err != nil {
		return "", err
	}

	name := strings.TrimSpace(string(output))
	if name == "" {
		return "", fmt.Errorf("currentDlogLink has an empty target")
	}
	return name, nil
}

func selectDlogFiles(allFiles []string, currentFile string, dlogDays int) ([]string, error) {
	if dlogDays < 0 {
		return nil, fmt.Errorf("DLOG_DAYS must be a non-negative integer")
	}

	currentFound := false
	for _, name := range allFiles {
		if name == currentFile {
			currentFound = true
			break
		}
	}
	if !currentFound {
		return nil, fmt.Errorf("currentDlogLink target %q is not a regular .dlog file in /data/log", currentFile)
	}

	if len(allFiles) < 50 || dlogDays == 0 {
		return append([]string(nil), allFiles...), nil
	}

	dates := make(map[string]time.Time, len(allFiles))
	var latest time.Time
	for _, name := range allFiles {
		date, err := parseDlogDate(name)
		if err != nil {
			return nil, err
		}
		dates[name] = date
		if latest.IsZero() || date.After(latest) {
			latest = date
		}
	}

	cutoff := latest.AddDate(0, 0, -dlogDays)
	var selected []string
	currentSelected := false
	for _, name := range allFiles {
		date := dates[name]
		if !date.Before(cutoff) && !date.After(latest) {
			selected = append(selected, name)
			if name == currentFile {
				currentSelected = true
			}
		}
	}
	if !currentSelected {
		selected = append(selected, currentFile)
	}

	sort.Strings(selected)
	return selected, nil
}

func parseDlogDate(name string) (time.Time, error) {
	if filepath.Ext(name) != ".dlog" {
		return time.Time{}, fmt.Errorf("invalid dlog filename %q", name)
	}

	parts := strings.Split(strings.TrimSuffix(name, ".dlog"), "_")
	if len(parts) != 4 || len(parts[0]) == 0 || len(parts[1]) != 8 || len(parts[2]) != 3 || len(parts[3]) != 6 ||
		!isDecimalDigits(parts[1]) || !isDecimalDigits(parts[2]) || !isDecimalDigits(parts[3]) {
		return time.Time{}, fmt.Errorf("invalid dlog filename %q", name)
	}

	date, err := time.Parse("20060102", parts[1])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date in dlog filename %q", name)
	}
	return date, nil
}

func isDecimalDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func (c *collector) collectSystemCommands() error {
	commands := []struct {
		filename string
		command  string
	}{
		{filename: "cat_etc_version.txt", command: "cat /etc/version"},
		{filename: "date.txt", command: "date"},
		{filename: "uname_a.txt", command: "uname -a"},
		{filename: "ifconfig.txt", command: "ifconfig"},
		{filename: "ip_a.txt", command: "ip a"},
		{filename: "ip_route.txt", command: "ip route"},
		{filename: "mount.txt", command: "mount"},
	}

	failed := false
	for _, item := range commands {
		log("Collecting command output: %s -> %s", item.command, item.filename)
		if err := c.captureRemoteCommand(filepath.Join(c.outputDir, item.filename), item.command); err != nil {
			warn("Command failed: %s", item.command)
			failed = true
		}
	}
	if failed {
		return fmt.Errorf("one or more system commands failed")
	}
	return nil
}

func (c *collector) collectNetworkManagerConnections() error {
	log("Copying /etc/NetworkManager/system-connections")
	return c.copyRemoteTree("/etc/NetworkManager/system-connections", filepath.Join(c.outputDir, "etc", "NetworkManager", "system-connections"))
}

func (c *collector) collectNetworkManagerDump() error {
	outputPath := filepath.Join(c.outputDir, "network_manager_dump.txt")
	log("Creating NetworkManager dump for interface eth0")
	output, err := c.ssh.CombinedOutput("sh -s -- eth0", []byte(networkManagerDumpScript))
	if writeErr := os.WriteFile(outputPath, output, 0o644); writeErr != nil {
		return writeErr
	}
	return err
}

func (c *collector) collectEtcDehydrated() error {
	log("Copying complete /etc/dehydrated tree")
	return c.copyRemoteTree("/etc/dehydrated", filepath.Join(c.outputDir, "etc", "dehydrated"))
}

func (c *collector) collectDataDehydrated() error {
	log("Copying complete /data/dehydrated tree")
	return c.copyRemoteTree("/data/dehydrated", filepath.Join(c.outputDir, "data", "dehydrated"))
}

func (c *collector) collectTmpDehydrated() error {
	log("Copying complete /tmp/dehydrated tree")
	return c.copyRemoteTree("/tmp/dehydrated", filepath.Join(c.outputDir, "tmp", "dehydrated"))
}

func (c *collector) captureRemoteCommand(outputPath string, command string) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}
	output, err := c.ssh.CombinedOutput(command, nil)
	if writeErr := os.WriteFile(outputPath, output, 0o644); writeErr != nil {
		return writeErr
	}
	return err
}
