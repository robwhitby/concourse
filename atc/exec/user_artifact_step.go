package exec

import (
	"context"
	"errors"
	"io"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/worker"
)

type UserArtifactStep struct {
	id         atc.PlanID
	name       string
	artifactID int
	delegate   BuildStepDelegate
}

func UserArtifact(id atc.PlanID, plan atc.UserArtifactPlan, delegate BuildStepDelegate) Step {
	return &UserArtifactStep{
		id:         id,
		name:       plan.Name,
		artifactID: plan.ArtifactID,
		delegate:   delegate,
	}
}

func (step *UserArtifactStep) Run(ctx context.Context, state RunState) error {
	logger := lagerctx.FromContext(ctx).WithData(lager.Data{
		"plan-id": step.id,
		"name":    step.name,
	})

	state.Artifacts().RegisterSource(worker.ArtifactName(step.name), streamSource{
		logger,
		step,
		state,
	})

	return nil
}

func (step *UserArtifactStep) Succeeded() bool {
	return true
}

type streamSource struct {
	logger lager.Logger
	step   *UserArtifactStep
	state  RunState
}

func (source streamSource) StreamTo(logger lager.Logger, dest worker.ArtifactDestination) error {
	pb := progress(string(source.step.name)+":", source.step.delegate.Stdout())

	return source.state.ReadUserInput(source.step.id, func(rc io.ReadCloser) error {
		pb.Start()
		defer pb.Finish()

		source.logger.Debug("reading-user-input")
		return dest.StreamIn(".", pb.NewProxyReader(rc))
	})
}

func (source streamSource) StreamFile(logger lager.Logger, path string) (io.ReadCloser, error) {
	return nil, errors.New("cannot stream single file from user artifact")
}

func (source streamSource) VolumeOn(lager.Logger, worker.Worker) (worker.Volume, bool, error) {
	return nil, false, nil
}
