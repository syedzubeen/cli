package root

import (
	"net/http"
	"sync"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/instill-ai/cli/pkg/cmd/factory"
	"github.com/instill-ai/cli/pkg/cmdutil"

	apiCmd "github.com/instill-ai/cli/pkg/cmd/api"
	authCmd "github.com/instill-ai/cli/pkg/cmd/auth"
	completionCmd "github.com/instill-ai/cli/pkg/cmd/completion"
	configCmd "github.com/instill-ai/cli/pkg/cmd/config"
	versionCmd "github.com/instill-ai/cli/pkg/cmd/version"
)

// NewCmdRoot initiates the Cobra command root
func NewCmdRoot(f *cmdutil.Factory, version, buildDate string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "instill <command> <subcommand> [flags]",
		Short: "Instill CLI",
		Long:  `Use Instill Pipeline from the command line.`,

		SilenceErrors: true,
		SilenceUsage:  true,
		Example: heredoc.Doc(`
			$ instill api pipelines
			$ instill config get editor
			$ instill auth login
		`),
		Annotations: map[string]string{
			"help:feedback": heredoc.Doc(`
				Please open an issue on https://github.com/instill-ai/cli.
			`),
			"help:environment": heredoc.Doc(`
				See 'instill help environment' for the list of supported environment variables.
			`),
		},
	}

	cmd.SetOut(f.IOStreams.Out)
	cmd.SetErr(f.IOStreams.ErrOut)

	cmd.PersistentFlags().Bool("help", false, "Show help for command")
	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		rootHelpFunc(f, cmd, args)
	})
	cmd.SetUsageFunc(rootUsageFunc)
	cmd.SetFlagErrorFunc(rootFlagErrorFunc)

	formattedVersion := versionCmd.Format(version, buildDate)
	cmd.SetVersionTemplate(formattedVersion)
	cmd.Version = formattedVersion
	cmd.Flags().Bool("version", false, "Show instill version")

	// Child commands
	cmd.AddCommand(versionCmd.NewCmdVersion(f, version, buildDate))
	cmd.AddCommand(authCmd.NewCmdAuth(f))
	cmd.AddCommand(configCmd.NewCmdConfig(f))
	cmd.AddCommand(completionCmd.NewCmdCompletion(f.IOStreams))

	// the `api` command should not inherit any extra HTTP headers
	bareHTTPCmdFactory := *f
	bareHTTPCmdFactory.HTTPClient = bareHTTPClient(f, version)

	cmd.AddCommand(apiCmd.NewCmdApi(&bareHTTPCmdFactory, nil))

	// Help topics
	cmd.AddCommand(NewHelpTopic("environment"))
	cmd.AddCommand(NewHelpTopic("formatting"))
	cmd.AddCommand(NewHelpTopic("mintty"))
	referenceCmd := NewHelpTopic("reference")
	referenceCmd.SetHelpFunc(referenceHelpFn(f.IOStreams))
	cmd.AddCommand(referenceCmd)

	cmdutil.DisableAuthCheck(cmd)

	// this needs to appear last:
	referenceCmd.Long = referenceLong(cmd)
	return cmd
}

func bareHTTPClient(f *cmdutil.Factory, version string) func() (*http.Client, error) {
	return func() (*http.Client, error) {
		cfg, err := f.Config()
		if err != nil {
			return nil, err
		}
		return factory.NewHTTPClient(f.IOStreams, cfg, version, false)
	}
}

type lazyLoadedHTTPClient struct {
	factory *cmdutil.Factory

	httpClientMu sync.RWMutex // guards httpClient
	httpClient   *http.Client
}

func (l *lazyLoadedHTTPClient) Do(req *http.Request) (*http.Response, error) {
	l.httpClientMu.RLock()
	httpClient := l.httpClient
	l.httpClientMu.RUnlock()

	if httpClient == nil {
		var err error
		l.httpClientMu.Lock()
		l.httpClient, err = l.factory.HTTPClient()
		l.httpClientMu.Unlock()
		if err != nil {
			return nil, err
		}
	}

	return l.httpClient.Do(req)
}
