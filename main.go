// jenklod-batman is a terminal UI for managing Jenkins jobs.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/krank56/jenklod-batman/internal/config"
	"github.com/krank56/jenklod-batman/internal/jenkins"
	"github.com/krank56/jenklod-batman/internal/notify"
	"github.com/krank56/jenklod-batman/internal/secret"
	"github.com/krank56/jenklod-batman/internal/ui"
	"github.com/krank56/jenklod-batman/internal/update"
)

var version = "dev"

// cmd is the name this was run as: jenklod-batman, or its short link jk.
// Commands we tell you to type use it.
var cmd = filepath.Base(os.Args[0])

// usage fills a command's usage text, where %[1]s is cmd.
func usage(text string) string { return fmt.Sprintf(text, cmd) }

func buildInfo() *debug.BuildInfo {
	bi, _ := debug.ReadBuildInfo()
	return bi
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, cmd+":", err)
		os.Exit(1)
	}
}

func run() error {
	defPath, err := config.DefaultPath()
	if err != nil {
		return err
	}
	current, fromBuildInfo := update.Current(version, buildInfo())
	if len(os.Args) > 1 {
		// Commands first: a macro with one of their names needs "run".
		switch a := os.Args[1]; {
		case a == "run":
			return runMacro(defPath, os.Args[2:])
		case a == "update":
			return runUpdate(current, fromBuildInfo, os.Args[2:])
		case a == "changelog":
			return runChangelog(os.Args[2:])
		case !strings.HasPrefix(a, "-"):
			return runMacro(defPath, os.Args[1:])
		}
	}
	var (
		cfgPath     = flag.String("config", defPath, "config file")
		noAnim      = flag.Bool("no-anim", false, "skip the splash and keep the cat still")
		setup       = flag.Bool("setup", false, "re-run the setup form (URL, user, token)")
		forget      = flag.Bool("forget-token", false, "remove the stored token from the keyring and exit")
		showVersion = flag.Bool("version", false, "print the version and exit")
	)
	flag.Usage = func() {
		fmt.Fprint(flag.CommandLine.Output(), usage("Usage: %[1]s [flags]\n"+
			"       %[1]s <macro> [VALUE] [KEY=VALUE…] [-y]   (run --list to list)\n"+
			"       %[1]s update [--check]\n"+
			"       %[1]s changelog [VERSION]\n\nFlags:\n"))
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVersion {
		fmt.Println("jenklod-batman", current)
		return nil
	}

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}

	if *forget {
		if !cfg.Complete() {
			return errors.New("no URL/user configured, nothing to forget")
		}
		if err := secret.Delete(cfg.URL, cfg.User); err != nil {
			return err
		}
		fmt.Println("Token removed from the " + secret.StoreName() + ".")
		return nil
	}

	var client *jenkins.Client
	if cfg.Complete() && !*setup {
		token, _, err := secret.Get(cfg.URL, cfg.User)
		switch {
		case err == nil:
			if client, err = jenkins.New(cfg.URL, cfg.User, token); err != nil {
				return err
			}
		case !errors.Is(err, secret.ErrMissing):
			return fmt.Errorf("reading token from the %s: %w", secret.StoreName(), err)
		}
	}

	updates, whatsNew := startupUpdates(cfg, current)
	m := ui.New(ui.Options{
		Config:   cfg,
		Client:   client,
		NoAnim:   *noAnim,
		WizardFn: wizardFunc(cfg),
		Command:  cmd,
		Updates:  updates,
		WhatsNew: whatsNew,
	})
	if !*noAnim {
		defer notify.Play(cfg.SplashMusic)()
	}
	_, err = tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

// wizardFunc checks the entered settings against Jenkins before saving
// anything, so a typo never ends up in the config or keyring.
func wizardFunc(cfg *config.Config) ui.WizardFunc {
	return func(url, user, token string) (*jenkins.Client, string, error) {
		if url == "" || user == "" {
			return nil, "", errors.New("URL and user are required")
		}
		// A token typed into the form wins over $JENKINS_TOKEN and is the
		// only one we store.
		typed := token != ""
		if !typed {
			t, _, err := secret.Get(url, user)
			if err != nil {
				return nil, "", errors.New("an API token is required")
			}
			token = t
		}
		c, err := jenkins.New(url, user, token)
		if err != nil {
			return nil, "", err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		who, err := c.Ping(ctx)
		if err != nil {
			return nil, "", err
		}
		if typed {
			if err := secret.Set(url, user, token); err != nil {
				return nil, "", fmt.Errorf("saving token to the %s: %w", secret.StoreName(), err)
			}
		}
		cfg.URL, cfg.User = url, user
		if err := cfg.Save(); err != nil {
			return nil, "", fmt.Errorf("saving config: %w", err)
		}
		return c, who, nil
	}
}
