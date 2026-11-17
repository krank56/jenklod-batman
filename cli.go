package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/krank56/jenklod-batman/internal/config"
	"github.com/krank56/jenklod-batman/internal/jenkins"
	"github.com/krank56/jenklod-batman/internal/macro"
	"github.com/krank56/jenklod-batman/internal/secret"
)

const runUsage = `Usage: %[1]s <macro> [VALUE] [KEY=VALUE…] [-y] [--config FILE]
       %[1]s run <macro> [VALUE] [KEY=VALUE…] [-y] [--config FILE]
       %[1]s run --list

Runs a macro made in the TUI (press m). VALUE fills the macro's argument
parameter, if it has one (e.g. BRANCH). KEY=VALUE overrides that parameter
in every build step that has it. On a terminal it shows the steps and asks
first; -y (or no terminal, e.g. cron) runs straight away. Exits non-zero if
a step fails.`

// runMacro is the `run` subcommand, also reached as `jk <macro>`.
func runMacro(cfgPath string, args []string) error {
	var (
		name, arg string
		hasArg    bool
		yes, list bool
		overrides = map[string]string{}
	)
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-y" || a == "--yes":
			yes = true
		case a == "-l" || a == "--list":
			list = true
		case a == "-h" || a == "--help":
			fmt.Println(usage(runUsage))
			return nil
		case a == "--config" || a == "-config":
			if i++; i >= len(args) {
				return errors.New("--config needs a file")
			}
			cfgPath = args[i]
		case strings.HasPrefix(a, "--config="), strings.HasPrefix(a, "-config="):
			_, cfgPath, _ = strings.Cut(a, "=")
		case name != "" && strings.Contains(a, "="):
			k, v, _ := strings.Cut(a, "=")
			if k == "" {
				return fmt.Errorf("bad override %q: want KEY=VALUE", a)
			}
			overrides[k] = v
		case strings.HasPrefix(a, "-"):
			return fmt.Errorf("unknown flag %s\n\n%s", a, usage(runUsage))
		case name == "":
			name = a
		case !hasArg:
			arg, hasArg = a, true
		default:
			return fmt.Errorf("unexpected %q: only one VALUE, other parameters are KEY=VALUE", a)
		}
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	if list {
		ms := cfg.Macros()
		if len(ms) == 0 {
			fmt.Println("No macros for " + cfg.URL + " yet: create one in the TUI with m.")
		}
		for _, m := range ms {
			usage := m.Name
			if m.Arg != "" {
				usage += " <" + m.Arg + ">"
			}
			fmt.Printf("%s (%d steps)\n", usage, len(m.Steps))
			for i, s := range m.Steps {
				fmt.Printf("  %d. %s\n", i+1, s)
			}
		}
		return nil
	}
	if name == "" {
		return errors.New(usage(runUsage))
	}
	mac, ok := cfg.Macro(name)
	if !ok {
		return fmt.Errorf("no macro called %q for %s (run --list)", name, cfg.URL)
	}
	if hasArg {
		if err := macro.SetArg(mac, arg, overrides); err != nil {
			return err
		}
	}
	if err := macro.CheckOverrides(mac, overrides); err != nil {
		return err
	}
	if !cfg.Complete() {
		return fmt.Errorf("not configured: run %s --setup first", cmd)
	}
	token, _, err := secret.Get(cfg.URL, cfg.User)
	if err != nil {
		return fmt.Errorf("reading token from the %s: %w", secret.StoreName(), err)
	}
	client, err := jenkins.New(cfg.URL, cfg.User, token)
	if err != nil {
		return err
	}

	fmt.Printf("Macro %s on %s:\n", mac.Name, client.Host())
	for i, s := range mac.Steps {
		fmt.Printf("  %d. %s\n", i+1, s)
	}
	for k, v := range overrides {
		fmt.Printf("  with %s=%s\n", k, v)
	}
	if !yes && isTerminal(os.Stdin) {
		fmt.Print("Run it? [y/N] ")
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		if a := strings.ToLower(strings.TrimSpace(line)); a != "y" && a != "yes" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	// Watch what it builds, so the TUI shows and notifies it next time.
	added := false
	for _, j := range mac.BuiltJobs() {
		added = cfg.Watch(j) || added
	}
	if added {
		if err := cfg.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: watching the jobs it builds: %v\n", err)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	r := &macro.Runner{
		Client: client,
		Report: func(e macro.Event) { fmt.Printf("[%d/%d] %s\n", e.Step, e.Total, e.Text) },
	}
	if err := r.Run(ctx, mac, overrides); err != nil {
		if errors.Is(err, context.Canceled) {
			return fmt.Errorf("interrupted at %w (builds already started keep running)", err)
		}
		return fmt.Errorf("macro %s failed at %w", mac.Name, err)
	}
	fmt.Println("✔ " + mac.Name + " done")
	return nil
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
