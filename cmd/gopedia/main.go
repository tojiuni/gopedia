// Gopedia CLI: HTTP client to the Fuego API and local server command.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"gopedia/internal/api"
)

func apiBase() string {
	b := strings.TrimSpace(os.Getenv("GOPEDIA_API_URL"))
	if b == "" {
		b = "http://127.0.0.1:18787"
	}
	return strings.TrimRight(b, "/")
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	root := &cobra.Command{
		Use:   "gopedia",
		Short: "Gopedia CLI (Fuego API client and local server)",
	}

	serverCmd := &cobra.Command{
		Use:   "server",
		Short: "Run the Gopedia Fuego HTTP API",
		RunE: func(cmd *cobra.Command, args []string) error {
			addr, _ := cmd.Flags().GetString("addr")
			if addr == "" {
				addr = os.Getenv("GOPEDIA_HTTP_ADDR")
			}
			if addr == "" {
				addr = "127.0.0.1:8787"
			}
			slog.Info("starting Gopedia API", "addr", addr)
			return api.Run(addr)
		},
	}
	serverCmd.Flags().String("addr", "", "listen address (default: GOPEDIA_HTTP_ADDR or 127.0.0.1:8787)")

	serviceCmd := &cobra.Command{
		Use:     "service",
		Short:   "Manage local services (alias: use `gopedia server`)",
		Aliases: []string{"svc"},
	}
	serviceStart := &cobra.Command{
		Use:   "start",
		Short: "Start the Fuego API server (same as `gopedia server`)",
		RunE:  serverCmd.RunE,
	}
	serviceStart.Flags().String("addr", "", "listen address (default: GOPEDIA_HTTP_ADDR or 127.0.0.1:8787)")
	serviceCmd.AddCommand(serviceStart)

	searchCmd := &cobra.Command{
		Use:   "search QUERY",
		Short: "Semantic search via GET /api/search",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			base := apiBase()
			q := url.QueryEscape(args[0])
			useJSON, _ := cmd.Flags().GetBool("json")
			u := base + "/api/search?q=" + q
			if useJSON {
				u += "&format=json"
				if d, _ := cmd.Flags().GetString("detail"); strings.TrimSpace(d) != "" {
					u += "&detail=" + url.QueryEscape(strings.TrimSpace(d))
				}
				if f, _ := cmd.Flags().GetString("fields"); strings.TrimSpace(f) != "" {
					u += "&fields=" + url.QueryEscape(strings.TrimSpace(f))
				}
			}
			client := &http.Client{Timeout: 6 * time.Minute}
			resp, err := client.Get(u)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("api %s: %s", resp.Status, string(body))
			}
			if useJSON {
				var v any
				if err := json.Unmarshal(body, &v); err != nil {
					return fmt.Errorf("decode response: %w", err)
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetEscapeHTML(false)
				return enc.Encode(v)
			}
			var out api.SearchResponse
			if err := json.Unmarshal(body, &out); err != nil {
				return fmt.Errorf("decode response: %w", err)
			}
			if out.Stderr != "" {
				fmt.Fprint(os.Stderr, out.Stderr)
			}
			if out.Error != "" {
				return fmt.Errorf("search error: %s", out.Error)
			}
			fmt.Println(out.Markdown)
			return nil
		},
	}
	searchCmd.Flags().Bool("json", false, "Print full JSON response (GET /api/search?format=json)")
	searchCmd.Flags().String("detail", "", "With --json: search detail preset (summary|standard|full); omit for full")
	searchCmd.Flags().String("fields", "", "With --json: comma-separated result keys (overrides --detail)")

	restoreCmd := &cobra.Command{
		Use:   "restore",
		Short: "Restore content via GET /api/restore",
		RunE: func(cmd *cobra.Command, args []string) error {
			base := apiBase()
			useJSON, _ := cmd.Flags().GetBool("json")
			l1ID, _ := cmd.Flags().GetString("l1-id")
			l2ID, _ := cmd.Flags().GetString("l2-id")
			l1ID = strings.TrimSpace(l1ID)
			l2ID = strings.TrimSpace(l2ID)
			if (l1ID == "" && l2ID == "") || (l1ID != "" && l2ID != "") {
				return fmt.Errorf("exactly one of --l1-id or --l2-id is required")
			}

			q := url.Values{}
			if useJSON {
				q.Set("format", "json")
			}
			if l1ID != "" {
				q.Set("l1_id", l1ID)
			}
			if l2ID != "" {
				q.Set("l2_id", l2ID)
			}
			u := base + "/api/restore?" + q.Encode()

			client := &http.Client{Timeout: 6 * time.Minute}
			resp, err := client.Get(u)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("api %s: %s", resp.Status, string(body))
			}
			if useJSON {
				var v any
				if err := json.Unmarshal(body, &v); err != nil {
					return fmt.Errorf("decode response: %w", err)
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetEscapeHTML(false)
				return enc.Encode(v)
			}
			var out api.RestoreResponse
			if err := json.Unmarshal(body, &out); err != nil {
				return fmt.Errorf("decode response: %w", err)
			}
			if out.Stderr != "" {
				fmt.Fprint(os.Stderr, out.Stderr)
			}
			if out.Error != "" {
				return fmt.Errorf("restore error: %s", out.Error)
			}
			fmt.Println(out.Markdown)
			return nil
		},
	}
	restoreCmd.Flags().String("l1-id", "", "knowledge_l1.id UUID (restore full L1 content)")
	restoreCmd.Flags().String("l2-id", "", "knowledge_l2.id UUID (restore code section)")
	restoreCmd.Flags().Bool("json", false, "Print full JSON response (GET /api/restore?format=json)")

	ingestCmd := &cobra.Command{
		Use:   "ingest PATH",
		Short: "Ingest markdown path via POST /api/ingest",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			base := apiBase()
			useJSON, _ := cmd.Flags().GetBool("json")
			payload, err := json.Marshal(map[string]string{"path": args[0]})
			if err != nil {
				return err
			}
			client := &http.Client{Timeout: 35 * time.Minute}
			resp, err := client.Post(base+"/api/ingest", "application/json", bytes.NewReader(payload))
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("api %s: %s", resp.Status, string(body))
			}
			if useJSON {
				var v any
				if err := json.Unmarshal(body, &v); err != nil {
					return fmt.Errorf("decode response: %w", err)
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetEscapeHTML(false)
				return enc.Encode(v)
			}
			var out api.IngestResponse
			if err := json.Unmarshal(body, &out); err != nil {
				return fmt.Errorf("decode response: %w", err)
			}
			if out.Stdout != "" {
				fmt.Print(out.Stdout)
			}
			if out.Stderr != "" {
				fmt.Fprint(os.Stderr, out.Stderr)
			}
			if !out.OK || out.Error != "" {
				if out.Error != "" {
					return fmt.Errorf("ingest error: %s", out.Error)
				}
				return fmt.Errorf("ingest failed")
			}
			return nil
		},
	}
	ingestCmd.Flags().Bool("json", false, "Print full JSON response from POST /api/ingest")

	projectCmd := &cobra.Command{
		Use:   "project",
		Short: "Project workspace helpers",
	}
	projectInit := &cobra.Command{
		Use:   "init",
		Short: "Initialize a Gopedia-linked workspace (placeholder)",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("gopedia project init: configure GOPEDIA_API_URL and run `gopedia server`, then `gopedia ingest <path>`.")
			return nil
		},
	}
	projectCmd.AddCommand(projectInit)

	root.AddCommand(serverCmd, serviceCmd, searchCmd, restoreCmd, ingestCmd, projectCmd)

	if err := root.Execute(); err != nil {
		slog.Error("gopedia", "err", err)
		os.Exit(1)
	}
}
