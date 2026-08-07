package collector

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
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
	return c.copyRemoteTree("/data/profiles", filepath.Join(c.outputDir, "data_profiles"))
}

func (c *collector) collectDlogs() error {
	dlogDir := filepath.Join(c.outputDir, "dlogs")
	if err := os.MkdirAll(dlogDir, 0o755); err != nil {
		return err
	}

	if _, err := c.ssh.CombinedOutput(`find /data/log -maxdepth 1 -type f -name "*.dlog" -print -quit | grep -q .`, nil); err != nil {
		warn("No .dlog files found in /data/log")
		return nil
	}

	log("Copying .dlog files from /data/log")
	command := `cd /data/log || exit 1
set --
for file in ./*.dlog; do
    [ -f "$file" ] || continue
    set -- "$@" "$file"
done
tar -cf - "$@"`
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
	return c.copyRemoteTree("/etc/NetworkManager/system-connections", filepath.Join(c.outputDir, "networkmanager_system_connections"))
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
