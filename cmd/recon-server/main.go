// Command recon-server serves the browser UI and API on the office network,
// handles sign-in, the Admin tab, uploads and the job queue, and supervises recon-engine.
//
// Logins, the job queue and the full API come in later specs. Today it serves the screens
// and a test API: upload JSON files, run the checks and list the rules (SPEC-004 part 6).
//
//	go build -tags ui -o build/ ./cmd/recon-engine ./cmd/recon-server
//	build/recon-server --listen 127.0.0.1:8080 --db build/ui.db
//
// It listens on this computer only unless another address is given, and it runs every
// check in a separate recon-engine process; it never evaluates a rule itself.
package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/api"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/web"
)

func main() {
	listen := flag.String("listen", defaultListen, "address to serve on: this computer only by default; this computer's LAN address (such as 192.168.1.10:8080) for the office network")
	hosts := flag.String("host", "", "computer names, separated by commas, that browsers may use to reach the server (such as office-pc); IP addresses and localhost always work")
	cfg := api.Config{}
	flag.StringVar(&cfg.DB, "db", "build/ui.db", "database file the engine writes")
	flag.StringVar(&cfg.Engine, "engine", defaultEngine(), "the recon-engine program")
	flag.StringVar(&cfg.Store, "store", "docs/Recon_Tool_YAML_Rule_Store/recon_yaml", "YAML rule store folder")
	flag.StringVar(&cfg.Rules, "rules", "content/rules", "engine rules folder")
	flag.StringVar(&cfg.Vocab, "vocab", "content/vocabulary", "fact vocabulary folder")
	flag.StringVar(&cfg.Profiles, "profiles", "content/profiles", "format profiles folder")
	flag.StringVar(&cfg.Office, "office-profiles", "", "folder for the formats taught on the mapping screen (default: a profiles folder beside the database)")
	flag.StringVar(&cfg.OfficeRules, "office-rules", "", "folder for the rules written in the office by the Rule Builder (default: an office-rules folder beside the database)")
	flag.StringVar(&cfg.Currency, "currency", "", "folder for usd-rate.yaml, the fixed rate for amounts in US dollars that the Admin tab sets (default: a currency folder beside the database)")
	flag.StringVar(&cfg.Comparisons, "comparisons", "content/comparisons/comparisons.yaml", "comparison types file")
	flag.StringVar(&cfg.Firm, "firm", "", "the firm's name, written on every sheet of an Excel export (SPEC-013)")
	flag.Parse()
	for _, h := range strings.Split(*hosts, ",") {
		if h = strings.TrimSpace(h); h != "" {
			cfg.Hosts = append(cfg.Hosts, h)
		}
	}
	if warning := exposure(*listen); warning != "" {
		fmt.Fprintf(os.Stderr, "recon-server: warning: %s\n", warning)
	}

	files, err := web.Files()
	if err != nil {
		fmt.Fprintf(os.Stderr, "recon-server: %v\n", err)
		os.Exit(2)
	}
	srv, err := api.New(cfg, files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "recon-server: %v\n", err)
		os.Exit(2)
	}
	if err := serve(*listen, srv.Handler()); err != nil {
		fmt.Fprintf(os.Stderr, "recon-server: %v\n", err)
		srv.Close()
		os.Exit(3)
	}
}

// defaultEngine is recon-engine in the same folder as this program.
func defaultEngine() string {
	name := "recon-engine"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	exe, err := os.Executable()
	if err != nil {
		return name
	}
	return filepath.Join(filepath.Dir(exe), name)
}

// defaultListen is this computer only (technical spec §B12.2): the server is never reached from
// the office network unless the operator asks for it with --listen.
const defaultListen = "127.0.0.1:8080"

// exposure warns when addr can be reached from other computers. Sign-in is in place (SPEC-012),
// but the connection is plain HTTP until HTTPS is decided (T-Q2), so the warning stays.
func exposure(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return ""
	}
	ip := net.ParseIP(host)
	switch {
	case host == "localhost" || ip != nil && ip.IsLoopback():
		return ""
	case host == "" || ip != nil && ip.IsUnspecified():
		host = "every network this computer is connected to"
	}
	return "the server can be reached on " + host + " over plain HTTP (HTTPS is not decided yet, T-Q2); anyone who reaches it can try to sign in, and a signed-in user can read the client figures they are allowed to see, so use it only on a trusted office network"
}

func serve(addr string, h http.Handler) error {
	srv := &http.Server{Addr: addr, Handler: h, ReadHeaderTimeout: 10 * time.Second}
	fmt.Fprintf(os.Stderr, "recon-server: open http://%s/ in a browser (Ctrl+C to stop)\n", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
