package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/spf13/cobra"

	"github.com/justwaters/sitrep/internal/api"
	"github.com/justwaters/sitrep/internal/config"
	"github.com/justwaters/sitrep/internal/sysd"
)

func newManagerTokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Manage worker enrollment tokens",
	}

	var dataDir string
	var ttl time.Duration
	create := &cobra.Command{
		Use:   "create",
		Short: "Create a one-time enrollment token for a new worker",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runManagerTokenCreate(cmd, dataDir, ttl)
		},
	}
	create.Flags().StringVar(&dataDir, "data-dir", config.ManagerDataDir, "directory holding the manager's config")
	create.Flags().DurationVar(&ttl, "ttl", 0, "token validity (default: 15m); the token must be used within this window")
	cmd.AddCommand(create)
	return cmd
}

func runManagerTokenCreate(cmd *cobra.Command, dataDir string, ttl time.Duration) error {
	cfg, err := config.LoadManagerConfig(config.ManagerConfigPath(dataDir))
	if err != nil {
		return fmt.Errorf("load manager config (has the manager been set up? run `sitrep manager start` first): %w", err)
	}

	reqBody, err := json.Marshal(api.TokenCreateRequest{TTLSeconds: int(ttl.Seconds())})
	if err != nil {
		return err
	}

	resp, err := http.Post("http://"+cfg.APIListenAddr+"/v1/tokens", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("contact manager at %s (is it running? try: systemctl status %s): %w",
			cfg.APIListenAddr, sysd.ManagerUnitName, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("token create failed (%s): %s", resp.Status, bytes.TrimSpace(body))
	}

	var out api.TokenCreateResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	w := cmd.OutOrStdout()
	fmt.Fprintln(w, "Enrollment token created. Give the worker operator all four of these values:")
	fmt.Fprintf(w, "  Manager address:  %s\n", out.EnrollAddr)
	fmt.Fprintf(w, "  Token:            %s\n", out.Token)
	fmt.Fprintf(w, "  Expires:          %s\n", time.Unix(out.ExpiresAt, 0).Format(time.RFC3339))
	fmt.Fprintf(w, "  Cert fingerprint: %s\n", out.ServerCertFingerprint)
	fmt.Fprintln(w, "\nThe token is single-use and expires at the time above. If the manager restarts before it's used, generate a new one.")
	return nil
}
