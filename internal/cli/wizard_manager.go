package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/justwaters/sitrep/internal/config"
	"github.com/justwaters/sitrep/internal/sysd"
	"github.com/justwaters/sitrep/internal/transport"
)

// runManagerWizard interactively collects the manager's listen addresses,
// enabled checks, and reporting interval, saves config.yaml, and installs
// the manager as a persistent systemd service.
func runManagerWizard(cmd *cobra.Command, dataDir, configPath string) (*config.ManagerConfig, error) {
	out := cmd.OutOrStdout()

	if os.Geteuid() != 0 {
		return nil, fmt.Errorf("run `sitrep manager start` as root (e.g. via sudo) — first-run setup installs a systemd service")
	}

	cfg := config.DefaultManagerConfig()
	cfg.DataDir = dataDir

	fmt.Fprintln(out, "Sitrep manager setup")
	fmt.Fprintln(out, "=====================")

	listenAddr := cfg.ListenAddr
	enrollAddr := cfg.EnrollAddr
	advertiseHost := cfg.AdvertiseHost
	apiAddr := cfg.APIListenAddr
	intervalStr := "60s"
	enabledChecks := append([]string(nil), transport.AllChecks...)

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Listen addresses").
				Description("Where this manager will listen for worker enrollment requests and status reports."),
			huh.NewInput().Title("Report listen address (mTLS, host:port)").Value(&listenAddr),
			huh.NewInput().Title("Enrollment listen address (mTLS bootstrap, host:port)").Value(&enrollAddr),
			huh.NewInput().Title("Advertised address workers should use to reach this manager (hostname or IP)").Value(&advertiseHost),
			huh.NewInput().Title("Local query API address (must be loopback)").Value(&apiAddr),
		),
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Which stats should worker reports include?").
				Options(
					huh.NewOption("Internet Access (ping 8.8.8.8)", transport.CheckInternet).Selected(true),
					huh.NewOption("Disk Space", transport.CheckDisk).Selected(true),
					huh.NewOption("DRAM Usage", transport.CheckDRAM).Selected(true),
					huh.NewOption("CPU Usage", transport.CheckCPU).Selected(true),
					huh.NewOption("Git Auth Status (gh CLI)", transport.CheckGitAuth).Selected(true),
				).
				Value(&enabledChecks),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("How often should workers report?").
				Description("Go duration syntax, e.g. 30s, 1m, 5m").
				Value(&intervalStr),
		),
	)
	if err := form.Run(); err != nil {
		return nil, fmt.Errorf("setup wizard: %w", err)
	}

	interval, err := time.ParseDuration(intervalStr)
	if err != nil || interval <= 0 {
		return nil, fmt.Errorf("invalid reporting interval %q", intervalStr)
	}

	cfg.ListenAddr = listenAddr
	cfg.EnrollAddr = enrollAddr
	cfg.AdvertiseHost = advertiseHost
	cfg.APIListenAddr = apiAddr
	cfg.EnabledChecks = enabledChecks
	cfg.IntervalSeconds = int(interval.Seconds())

	if err := cfg.Save(configPath); err != nil {
		return nil, fmt.Errorf("save manager config: %w", err)
	}

	fmt.Fprintln(out, "\nInstalling the sitrep-manager systemd service...")

	if err := sysd.EnsureSystemUser(); err != nil {
		return nil, fmt.Errorf("create sitrep system user: %w", err)
	}
	if err := sysd.ChownDataDir(cfg.DataDir); err != nil {
		return nil, fmt.Errorf("set data directory ownership: %w", err)
	}

	binaryPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve sitrep binary path: %w", err)
	}

	unitContent, err := sysd.RenderUnit(sysd.ManagerUnitTemplate, sysd.UnitParams{
		BinaryPath: binaryPath,
		ConfigPath: configPath,
		DataDir:    cfg.DataDir,
	})
	if err != nil {
		return nil, err
	}
	if err := sysd.InstallUnit(sysd.ManagerUnitName, unitContent); err != nil {
		return nil, fmt.Errorf("install systemd service: %w", err)
	}

	fmt.Fprintf(out, "\nDone. The manager is now running in the background as %s.\n", sysd.ManagerUnitName)
	fmt.Fprintf(out, "Check its status any time with: systemctl status %s\n", sysd.ManagerUnitName)
	fmt.Fprintln(out, "\nNext: run `sitrep manager token create` to enroll a worker.")

	return cfg, nil
}
