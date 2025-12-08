// Package app wires together services, coordinates agents, and manages
// application lifecycle.
package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/fantasy"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/agent/capability"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/format"
	"github.com/charmbracelet/crush/internal/freshness"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/log"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/memory"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/plugin"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/runtime"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/shell"
	"github.com/charmbracelet/crush/internal/telemetry"
	"github.com/charmbracelet/crush/internal/term"
	"github.com/charmbracelet/crush/internal/tui/components/anim"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/crush/internal/update"
	"github.com/charmbracelet/crush/internal/version"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/charmtone"
)

type App struct {
	Sessions    session.Service
	Messages    message.Service
	History     history.Service
	Permissions permission.Service

	AgentCoordinator agent.Coordinator

	Telemetry   *telemetry.Recorder
	Freshness   *freshness.Service
	Memory      *memory.Service
	Plugins     *plugin.Host
	CapRegistry *capability.Registry

	LSPClients *csync.Map[string, *lsp.Client]

	config     *config.Config
	registrars []runtime.Registrar

	serviceEventsWG *sync.WaitGroup
	eventsCtx       context.Context
	events          chan tea.Msg
	tuiWG           *sync.WaitGroup

	// global context and cleanup functions
	globalCtx    context.Context
	cleanupFuncs []func() error
}

// New initializes a new applcation instance.
func New(ctx context.Context, conn *sql.DB, cfg *config.Config) (*App, error) {
	q := db.New(conn)
	sessions := session.NewService(q)
	messages := message.NewService(q)
	files := history.NewService(q, conn)
	skipPermissionsRequests := cfg.Permissions != nil && cfg.Permissions.SkipRequests
	allowedTools := []string{}
	if cfg.Permissions != nil && cfg.Permissions.AllowedTools != nil {
		allowedTools = cfg.Permissions.AllowedTools
	}
	telemetryRecorder := telemetry.NewRecorder()
	fresh := freshness.New()
	freshnessPath := filepath.Join(cfg.Options.DataDirectory, "freshness.json")
	_ = fresh.Load(freshnessPath)
	mem := memory.NewService(cfg.Options.DataDirectory)
	pluginHost := plugin.NewHost(plugin.PersistentPath(cfg.Options.DataDirectory, cfg.WorkingDir()), cfg.WorkingDir())
	capRegistry := capability.NewRegistry()
	appRegistrars := runtime.DefaultRegistrarsV2()
	if path := cfg.Options.ToolsManifest; path != "" {
		appRegistrars.Tools = append([]runtime.Registrar{runtime.ToolsManifestRegistrar{Path: path}}, appRegistrars.Tools...)
	} else if manifest := runtime.ResolveToolsManifest(cfg.ConfigDir()); manifest != "" {
		appRegistrars.Tools = append([]runtime.Registrar{runtime.ToolsManifestRegistrar{Path: manifest}}, appRegistrars.Tools...)
	}
	if path := cfg.Options.PluginsManifest; path != "" {
		appRegistrars.Plugins = append([]runtime.Registrar{runtime.PluginRegistryRegistrar{Path: path}}, appRegistrars.Plugins...)
	} else if manifest := runtime.ResolvePluginsManifest(cfg.ConfigDir()); manifest != "" {
		appRegistrars.Plugins = append([]runtime.Registrar{runtime.PluginRegistryRegistrar{Path: manifest}}, appRegistrars.Plugins...)
	}

	app := &App{
		Sessions:    sessions,
		Messages:    messages,
		History:     files,
		Permissions: permission.NewPermissionService(cfg.WorkingDir(), cfg.Options.DataDirectory, skipPermissionsRequests, allowedTools),
		Telemetry:   telemetryRecorder,
		Freshness:   fresh,
		Memory:      mem,
		Plugins:     pluginHost,
		CapRegistry: capRegistry,
		LSPClients:  csync.NewMap[string, *lsp.Client](),

		globalCtx: ctx,

		config: cfg,

		events:          make(chan tea.Msg, 100),
		serviceEventsWG: &sync.WaitGroup{},
		tuiWG:           &sync.WaitGroup{},
		registrars:      appRegistrars.Flatten(),
	}

	app.setupEvents()

	// Initialize LSP clients in the background.
	app.initLSPClients(ctx)

	// Check for updates in the background.
	go app.checkForUpdates(ctx)

	// Feed freshness with history events.
	go func() {
		sub := files.Subscribe(ctx)
		for evt := range sub {
			if evt.Payload.Path != "" {
				app.Freshness.Record(evt.Payload.SessionID, evt.Payload.Path)
			}
		}
	}()

	// Seed freshness with configured context paths.
	for _, p := range cfg.Options.ContextPaths {
		app.Freshness.RecordWorkspace(p)
	}
	if cfg.Options.InitializeAs != "" {
		agentsPath := filepath.Join(cfg.WorkingDir(), cfg.Options.InitializeAs)
		if _, err := os.Stat(agentsPath); err == nil {
			app.Freshness.RecordWorkspace(agentsPath)
		}
	}
	// Scan .crush for markdown metadata.
	if info, err := os.ReadDir(filepath.Join(cfg.WorkingDir(), ".crush")); err == nil {
		for _, entry := range info {
			if entry.IsDir() {
				continue
			}
			if strings.HasSuffix(entry.Name(), ".md") {
				app.Freshness.RecordWorkspace(filepath.Join(cfg.WorkingDir(), ".crush", entry.Name()))
			}
		}
	}
	// Scan for AGENTS-like files and seed freshness.
	for _, candidate := range []string{"AGENTS.md", "CRUSH.md", "CLAUDE.md"} {
		path := filepath.Join(cfg.WorkingDir(), candidate)
		if _, err := os.Stat(path); err == nil {
			app.Freshness.RecordWorkspace(path)
		}
	}

	// Run registrars after core services are ready.
	regErr := runtime.RunRegistrars(ctx, app.registrars, runtime.Services{
		Config:        cfg,
		ConfigPaths:   runtime.DefaultConfigPaths(cfg.WorkingDir()),
		Permissions:   app.Permissions,
		Telemetry:     telemetryRecorder,
		Freshness:     fresh,
		PluginHost:    pluginHost,
		ToolsRegistry: capRegistry,
		WorkingDir:    cfg.WorkingDir(),
		ConfigDir:     cfg.ConfigDir(),
		MCPInit: func(ctx context.Context) {
			slog.Info("Initializing MCP clients")
			mcp.Initialize(ctx, app.Permissions, cfg)
		},
	})
	if regErr != nil {
		return nil, regErr
	}

	// cleanup database upon app shutdown
	app.cleanupFuncs = append(app.cleanupFuncs, conn.Close, mcp.Close, func() error {
		return fresh.Save(freshnessPath)
	})

	// TODO: remove the concept of agent config, most likely.
	if !cfg.IsConfigured() {
		slog.Warn("No agent configuration found")
		return app, nil
	}
	if err := app.InitCoderAgent(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize coder agent: %w", err)
	}
	return app, nil
}

// Config returns the application configuration.
func (app *App) Config() *config.Config {
	return app.config
}

// RunNonInteractive runs the application in non-interactive mode with the
// given prompt, printing to stdout.
func (app *App) RunNonInteractive(ctx context.Context, output io.Writer, prompt string, quiet bool) error {
	slog.Info("Running in non-interactive mode")

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var spinner *format.Spinner
	if !quiet {
		t := styles.CurrentTheme()

		// Detect background color to set the appropriate color for the
		// spinner's 'Generating...' text. Without this, that text would be
		// unreadable in light terminals.
		hasDarkBG := true
		if f, ok := output.(*os.File); ok {
			hasDarkBG = lipgloss.HasDarkBackground(os.Stdin, f)
		}
		defaultFG := lipgloss.LightDark(hasDarkBG)(charmtone.Pepper, t.FgBase)

		spinner = format.NewSpinner(ctx, cancel, anim.Settings{
			Size:        10,
			Label:       "Generating",
			LabelColor:  defaultFG,
			GradColorA:  t.Primary,
			GradColorB:  t.Secondary,
			CycleColors: true,
		})
		spinner.Start()
	}

	// Helper function to stop spinner once.
	stopSpinner := func() {
		if !quiet && spinner != nil {
			spinner.Stop()
			spinner = nil
		}
	}
	defer stopSpinner()

	const maxPromptLengthForTitle = 100
	const titlePrefix = "Non-interactive: "
	var titleSuffix string

	if len(prompt) > maxPromptLengthForTitle {
		titleSuffix = prompt[:maxPromptLengthForTitle] + "..."
	} else {
		titleSuffix = prompt
	}
	title := titlePrefix + titleSuffix

	sess, err := app.Sessions.Create(ctx, title)
	if err != nil {
		return fmt.Errorf("failed to create session for non-interactive mode: %w", err)
	}
	slog.Info("Created session for non-interactive run", "session_id", sess.ID)

	// Automatically approve all permission requests for this non-interactive
	// session.
	app.Permissions.AutoApproveSession(sess.ID)

	type response struct {
		result *fantasy.AgentResult
		err    error
	}
	done := make(chan response, 1)

	go func(ctx context.Context, sessionID, prompt string) {
		result, err := app.AgentCoordinator.Run(ctx, sess.ID, prompt)
		if err != nil {
			done <- response{
				err: fmt.Errorf("failed to start agent processing stream: %w", err),
			}
		}
		done <- response{
			result: result,
		}
	}(ctx, sess.ID, prompt)

	messageEvents := app.Messages.Subscribe(ctx)
	messageReadBytes := make(map[string]int)
	supportsProgressBar := term.SupportsProgressBar()

	defer func() {
		if supportsProgressBar {
			_, _ = fmt.Fprintf(os.Stderr, ansi.ResetProgressBar)
		}

		// Always print a newline at the end. If output is a TTY this will
		// prevent the prompt from overwriting the last line of output.
		_, _ = fmt.Fprintln(output)
	}()

	for {
		if supportsProgressBar {
			// HACK: Reinitialize the terminal progress bar on every iteration so
			// it doesn't get hidden by the terminal due to inactivity.
			_, _ = fmt.Fprintf(os.Stderr, ansi.SetIndeterminateProgressBar)
		}

		select {
		case result := <-done:
			stopSpinner()
			if result.err != nil {
				if errors.Is(result.err, context.Canceled) || errors.Is(result.err, agent.ErrRequestCancelled) {
					slog.Info("Non-interactive: agent processing cancelled", "session_id", sess.ID)
					return nil
				}
				return fmt.Errorf("agent processing failed: %w", result.err)
			}
			return nil

		case event := <-messageEvents:
			msg := event.Payload
			if msg.SessionID == sess.ID && msg.Role == message.Assistant && len(msg.Parts) > 0 {
				stopSpinner()

				content := msg.Content().String()
				readBytes := messageReadBytes[msg.ID]

				if len(content) < readBytes {
					slog.Error("Non-interactive: message content is shorter than read bytes", "message_length", len(content), "read_bytes", readBytes)
					return fmt.Errorf("message content is shorter than read bytes: %d < %d", len(content), readBytes)
				}

				part := content[readBytes:]
				fmt.Fprint(output, part)
				messageReadBytes[msg.ID] = len(content)
			}

		case <-ctx.Done():
			stopSpinner()
			return ctx.Err()
		}
	}
}

func (app *App) UpdateAgentModel(ctx context.Context) error {
	return app.AgentCoordinator.UpdateModels(ctx)
}

func (app *App) setupEvents() {
	ctx, cancel := context.WithCancel(app.globalCtx)
	app.eventsCtx = ctx
	setupSubscriber(ctx, app.serviceEventsWG, "sessions", app.Sessions.Subscribe, app.events)
	setupSubscriber(ctx, app.serviceEventsWG, "messages", app.Messages.Subscribe, app.events)
	setupSubscriber(ctx, app.serviceEventsWG, "permissions", app.Permissions.Subscribe, app.events)
	setupSubscriber(ctx, app.serviceEventsWG, "permissions-notifications", app.Permissions.SubscribeNotifications, app.events)
	setupSubscriber(ctx, app.serviceEventsWG, "history", app.History.Subscribe, app.events)
	setupSubscriber(ctx, app.serviceEventsWG, "mcp", mcp.SubscribeEvents, app.events)
	setupSubscriber(ctx, app.serviceEventsWG, "lsp", SubscribeLSPEvents, app.events)
	setupSubscriber(ctx, app.serviceEventsWG, "telemetry", app.Telemetry.Subscribe, app.events)
	cleanupFunc := func() error {
		cancel()
		app.serviceEventsWG.Wait()
		return nil
	}
	app.cleanupFuncs = append(app.cleanupFuncs, cleanupFunc)
}

func setupSubscriber[T any](
	ctx context.Context,
	wg *sync.WaitGroup,
	name string,
	subscriber func(context.Context) <-chan pubsub.Event[T],
	outputCh chan<- tea.Msg,
) {
	wg.Go(func() {
		subCh := subscriber(ctx)
		for {
			select {
			case event, ok := <-subCh:
				if !ok {
					slog.Debug("subscription channel closed", "name", name)
					return
				}
				var msg tea.Msg = event
				select {
				case outputCh <- msg:
				case <-time.After(2 * time.Second):
					slog.Warn("message dropped due to slow consumer", "name", name)
				case <-ctx.Done():
					slog.Debug("subscription cancelled", "name", name)
					return
				}
			case <-ctx.Done():
				slog.Debug("subscription cancelled", "name", name)
				return
			}
		}
	})
}

func (app *App) InitCoderAgent(ctx context.Context) error {
	coderAgentCfg := app.config.Agents[config.AgentCoder]
	if coderAgentCfg.ID == "" {
		return fmt.Errorf("coder agent configuration is missing")
	}
	var err error
	app.AgentCoordinator, err = agent.NewCoordinator(
		ctx,
		app.config,
		app.Sessions,
		app.Messages,
		app.Permissions,
		app.History,
		app.Telemetry,
		app.Freshness,
		app.CapRegistry,
		app.LSPClients,
		app.Memory,
	)
	if err != nil {
		slog.Error("Failed to create coder agent", "err", err)
		return err
	}
	return nil
}

// Subscribe sends events to the TUI as tea.Msgs.
func (app *App) Subscribe(program *tea.Program) {
	defer log.RecoverPanic("app.Subscribe", func() {
		slog.Info("TUI subscription panic: attempting graceful shutdown")
		program.Quit()
	})

	app.tuiWG.Add(1)
	tuiCtx, tuiCancel := context.WithCancel(app.globalCtx)
	app.cleanupFuncs = append(app.cleanupFuncs, func() error {
		slog.Debug("Cancelling TUI message handler")
		tuiCancel()
		app.tuiWG.Wait()
		return nil
	})
	defer app.tuiWG.Done()

	for {
		select {
		case <-tuiCtx.Done():
			slog.Debug("TUI message handler shutting down")
			return
		case msg, ok := <-app.events:
			if !ok {
				slog.Debug("TUI message channel closed")
				return
			}
			program.Send(msg)
		}
	}
}

// Shutdown performs a graceful shutdown of the application.
func (app *App) Shutdown() {
	start := time.Now()
	defer func() { slog.Info("Shutdown took " + time.Since(start).String()) }()
	var wg sync.WaitGroup
	if app.AgentCoordinator != nil {
		wg.Go(func() {
			app.AgentCoordinator.CancelAll()
		})
	}

	// Kill all background shells.
	wg.Go(func() {
		shell.GetBackgroundShellManager().KillAll()
	})

	// Shutdown all LSP clients.
	for name, client := range app.LSPClients.Seq2() {
		wg.Go(func() {
			shutdownCtx, cancel := context.WithTimeout(app.globalCtx, 5*time.Second)
			defer cancel()
			if err := client.Close(shutdownCtx); err != nil {
				slog.Error("Failed to shutdown LSP client", "name", name, "error", err)
			}
		})
	}

	// Call call cleanup functions.
	for _, cleanup := range app.cleanupFuncs {
		if cleanup != nil {
			wg.Go(func() {
				if err := cleanup(); err != nil {
					slog.Error("Failed to cleanup app properly on shutdown", "error", err)
				}
			})
		}
	}
	wg.Wait()
}

// checkForUpdates checks for available updates.
func (app *App) checkForUpdates(ctx context.Context) {
	checkCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	info, err := update.Check(checkCtx, version.Version, update.Default)
	if err != nil || !info.Available() {
		return
	}
	app.events <- pubsub.UpdateAvailableMsg{
		CurrentVersion: info.Current,
		LatestVersion:  info.Latest,
		IsDevelopment:  info.IsDevelopment(),
	}
}
