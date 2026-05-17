package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/Priyasharma620064/kubedriftguard/internal/agents"
	"github.com/Priyasharma620064/kubedriftguard/internal/config"
	"github.com/Priyasharma620064/kubedriftguard/internal/gitsync"
	"github.com/Priyasharma620064/kubedriftguard/internal/version"

	driftguardv1alpha1 "github.com/Priyasharma620064/kubedriftguard/api/v1alpha1"
	driftcontroller "github.com/Priyasharma620064/kubedriftguard/internal/controller"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var (
	scheme   = runtime.NewScheme()
	cfgFile  string
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(driftguardv1alpha1.AddToScheme(scheme))
}

func main() {
	rootCmd := newRootCmd()
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "kubedriftguard",
		Short: "Real-time Kubernetes configuration drift detection and self-healing engine",
		Long: `KubeDriftGuard is a Kubernetes controller that continuously watches
live cluster state against declared GitOps manifests, detects configuration
drift in real-time, and triggers self-healing workflows with AGENTS.md-driven
remediation strategies.`,
	}

	root.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path (default: ./config.yaml)")

	root.AddCommand(newControllerCmd())
	root.AddCommand(newVersionCmd())
	root.AddCommand(newValidateCmd())

	return root
}

func newControllerCmd() *cobra.Command {
	var (
		metricsAddr string
		healthAddr  string
		dryRun      bool
	)

	cmd := &cobra.Command{
		Use:   "controller",
		Short: "Start the KubeDriftGuard controller",
		Long:  "Start the Kubernetes controller that watches DriftPolicy resources and performs drift detection.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runController(metricsAddr, healthAddr, dryRun)
		},
	}

	cmd.Flags().StringVar(&metricsAddr, "metrics-addr", ":8080", "The address for the Prometheus metrics endpoint")
	cmd.Flags().StringVar(&healthAddr, "health-addr", ":8081", "The address for the health probe endpoint")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Log remediation actions without applying them")

	return cmd
}

func runController(metricsAddr, healthAddr string, dryRun bool) error {
	// Setup logging
	ctrllog.SetLogger(zap.New(zap.UseDevMode(true)))
	logger := ctrllog.Log.WithName("setup")

	// Load configuration
	cfg, err := config.Load(cfgFile)
	if err != nil {
		logger.Error(err, "failed to load configuration")
		return err
	}

	// Parse AGENTS.md
	agentsCfg, err := agents.ParseFile(cfg.AgentsFile)
	if err != nil {
		logger.Error(err, "failed to parse AGENTS.md")
		return err
	}

	// Initialize Git syncer
	syncer, err := gitsync.NewSyncer(cfg.GitSync.CacheDir, cfg.GitSync.Timeout)
	if err != nil {
		logger.Error(err, "failed to initialize git syncer")
		return err
	}

	// Create controller manager
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: healthAddr,
	})
	if err != nil {
		logger.Error(err, "unable to create manager")
		return err
	}

	// Setup health checks
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		logger.Error(err, "unable to set up health check")
		return err
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		logger.Error(err, "unable to set up ready check")
		return err
	}

	// Setup reconciler
	reconciler := &driftcontroller.DriftPolicyReconciler{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		GitSyncer:    syncer,
		AgentsConfig: agentsCfg,
		DryRun:       dryRun || cfg.Remediation.DryRun,
		MaxRetries:   cfg.Remediation.MaxRetries,
	}

	if err := reconciler.SetupWithManager(mgr); err != nil {
		logger.Error(err, "unable to create controller")
		return err
	}

	logger.Info("starting KubeDriftGuard controller",
		"version", version.Version,
		"metrics", metricsAddr,
		"health", healthAddr,
		"dryRun", dryRun,
	)

	return mgr.Start(ctrl.SetupSignalHandler())
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("KubeDriftGuard\n")
			fmt.Printf("  Version:    %s\n", version.Version)
			fmt.Printf("  Git Commit: %s\n", version.GitCommit)
			fmt.Printf("  Build Date: %s\n", version.BuildDate)
		},
	}
}

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration and AGENTS.md",
		Long:  "Parse and validate the configuration file and AGENTS.md without starting the controller.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load config
			cfg, err := config.Load(cfgFile)
			if err != nil {
				return fmt.Errorf("config validation failed: %w", err)
			}
			fmt.Printf("✓ Configuration loaded successfully\n")
			fmt.Printf("  Log level:      %s\n", cfg.LogLevel)
			fmt.Printf("  Scan interval:  %s\n", cfg.ScanInterval)
			fmt.Printf("  Metrics addr:   %s\n", cfg.MetricsAddr)
			fmt.Printf("  Remediation:    enabled=%t dry_run=%t\n", cfg.Remediation.Enabled, cfg.Remediation.DryRun)

			// Parse AGENTS.md
			agentsCfg, err := agents.ParseFile(cfg.AgentsFile)
			if err != nil {
				return fmt.Errorf("AGENTS.md validation failed: %w", err)
			}
			fmt.Printf("\n✓ AGENTS.md parsed successfully\n")
			fmt.Printf("  Protected namespaces: %v\n", agentsCfg.ProtectedNamespaces)
			fmt.Printf("  Skip annotation:      %s\n", agentsCfg.SkipAnnotation)
			fmt.Printf("  Strategies:\n")
			for sev, action := range agentsCfg.RemediationStrategies {
				fmt.Printf("    %s → %s\n", sev, action)
			}
			fmt.Printf("  Boundaries:           %d rules\n", len(agentsCfg.Boundaries))

			// Test Git syncer initialization
			syncer, err := gitsync.NewSyncer(cfg.GitSync.CacheDir, cfg.GitSync.Timeout)
			if err != nil {
				return fmt.Errorf("git syncer validation failed: %w", err)
			}
			_ = syncer
			fmt.Printf("\n✓ Git syncer initialized (cache: %s)\n", cfg.GitSync.CacheDir)

			fmt.Printf("\n✅ All validations passed\n")
			return nil
		},
	}
}

// Ensure unused imports are used.
var (
	_ = time.Now
	_ = log.Printf
)
