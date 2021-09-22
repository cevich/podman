package containers

import (
	"fmt"

	"github.com/containers/common/pkg/completion"
	"github.com/containers/podman/v3/cmd/podman/common"
	"github.com/containers/podman/v3/cmd/podman/registry"
	"github.com/containers/podman/v3/cmd/podman/utils"
	"github.com/containers/podman/v3/cmd/podman/validate"
	"github.com/containers/podman/v3/pkg/domain/entities"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	cleanupDescription = `
   podman container cleanup

   Cleans up mount points and network stacks on one or more containers from the host. The container name or ID can be used. This command is used internally when running containers, but can also be used if container cleanup has failed when a container exits.
`
	cleanupCommand = &cobra.Command{
		Annotations: map[string]string{registry.EngineMode: registry.ABIMode},
		Use:         "cleanup [options] CONTAINER [CONTAINER...]",
		Short:       "Cleanup network and mountpoints of one or more containers",
		Long:        cleanupDescription,
		RunE:        cleanup,
		Args: func(cmd *cobra.Command, args []string) error {
			return validate.CheckAllLatestAndCIDFile(cmd, args, false, false)
		},
		ValidArgsFunction: common.AutocompleteContainersExited,
		Example: `podman container cleanup --latest
  podman container cleanup ctrID1 ctrID2 ctrID3
  podman container cleanup --all`,
	}
)

var (
	cleanupOptions entities.ContainerCleanupOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Parent:  containerCmd,
		Command: cleanupCommand,
	})
	flags := cleanupCommand.Flags()
	flags.BoolVarP(&cleanupOptions.All, "all", "a", false, "Cleans up all containers")

	execFlagName := "exec"
	flags.StringVar(&cleanupOptions.Exec, execFlagName, "", "Clean up the given exec session instead of the container")
	_ = cleanupCommand.RegisterFlagCompletionFunc(execFlagName, completion.AutocompleteNone)

	flags.BoolVar(&cleanupOptions.Remove, "rm", false, "After cleanup, remove the container entirely")
	flags.BoolVar(&cleanupOptions.RemoveImage, "rmi", false, "After cleanup, remove the image entirely")
	validate.AddLatestFlag(cleanupCommand, &cleanupOptions.Latest)
}

func cleanup(cmd *cobra.Command, args []string) error {
	var (
		errs utils.OutputErrors
	)

	if cleanupOptions.Exec != "" {
		switch {
		case cleanupOptions.All:
			return errors.Errorf("exec and all options conflict")
		case len(args) > 1:
			return errors.Errorf("cannot use exec option when more than one container is given")
		case cleanupOptions.RemoveImage:
			return errors.Errorf("exec and rmi options conflict")
		}
	}

	responses, err := registry.ContainerEngine().ContainerCleanup(registry.GetContext(), args, cleanupOptions)
	if err != nil {
		// `podman container cleanup` is almost always run in the
		// background. Our only way of relaying information to the user
		// is via syslog.
		// As such, we need to logrus.Errorf our errors to ensure they
		// are properly printed if --syslog is set.
		logrus.Errorf("Running container cleanup: %v", err)
		return err
	}
	for _, r := range responses {
		if r.CleanErr == nil && r.RmErr == nil && r.RmiErr == nil {
			fmt.Println(r.Id)
			continue
		}
		if r.RmErr != nil {
			logrus.Errorf("Removing container: %v", r.RmErr)
			errs = append(errs, r.RmErr)
		}
		if r.RmiErr != nil {
			logrus.Errorf("Removing image: %v", r.RmiErr)
			errs = append(errs, r.RmiErr)
		}
		if r.CleanErr != nil {
			logrus.Errorf("Cleaning up container: %v", r.CleanErr)
			errs = append(errs, r.CleanErr)
		}
	}
	return errs.PrintErrors()
}
