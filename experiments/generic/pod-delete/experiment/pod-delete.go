package experiment

import (
	"context"
	"os"

	"github.com/litmuschaos/chaos-operator/api/litmuschaos/v1alpha1"
	litmusLIB "github.com/litmuschaos/litmus-go/chaoslib/litmus/pod-delete/lib"
	"github.com/litmuschaos/litmus-go/pkg/clients"
	"github.com/litmuschaos/litmus-go/pkg/events"
	experimentEnv "github.com/litmuschaos/litmus-go/pkg/generic/pod-delete/environment"
	experimentTypes "github.com/litmuschaos/litmus-go/pkg/generic/pod-delete/types"
	"github.com/litmuschaos/litmus-go/pkg/log"
	"github.com/litmuschaos/litmus-go/pkg/probe"
	"github.com/litmuschaos/litmus-go/pkg/result"
	"github.com/litmuschaos/litmus-go/pkg/status"
	"github.com/litmuschaos/litmus-go/pkg/types"
	"github.com/litmuschaos/litmus-go/pkg/utils/common"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// PodDelete inject the pod-delete chaos
func PodDelete(ctx context.Context, clients clients.ClientSets) {
	span := trace.SpanFromContext(ctx)

	experimentsDetails := experimentTypes.ExperimentDetails{}
	resultDetails := types.ResultDetails{}
	eventsDetails := types.EventDetails{}
	chaosDetails := types.ChaosDetails{}

	//Fetching all the ENV passed from the runner pod
	log.WithContext(ctx).Infof("[PreReq]: Getting the ENV for the %v experiment", os.Getenv("EXPERIMENT_NAME"))
	experimentEnv.GetENV(&experimentsDetails)

	// Initialize the chaos attributes
	types.InitialiseChaosVariables(&chaosDetails)

	// Initialize Chaos Result Parameters
	types.SetResultAttributes(&resultDetails, chaosDetails)

	if experimentsDetails.EngineName != "" {
		// Get values from chaosengine. Bail out upon error, as we haven't entered exp business logic yet
		if err := types.GetValuesFromChaosEngine(&chaosDetails, clients, &resultDetails); err != nil {
			log.WithContext(ctx).Errorf("Unable to initialize the probes, err: %v", err)
			span.SetStatus(codes.Error, "Unable to initialize the probes")
			span.RecordError(err)
			return
		}
	}

	//Updating the chaos result in the beginning of experiment
	log.WithContext(ctx).Infof("[PreReq]: Updating the chaos result of %v experiment (SOT)", experimentsDetails.ExperimentName)
	if err := result.ChaosResult(&chaosDetails, clients, &resultDetails, "SOT"); err != nil {
		log.WithContext(ctx).Errorf("Unable to create the chaosresult, err: %v", err)
		result.RecordAfterFailure(&chaosDetails, &resultDetails, err, clients, &eventsDetails)
		span.SetStatus(codes.Error, "Unable to create the chaosresult")
		span.RecordError(err)
		return
	}

	// Set the chaos result uid
	if err := result.SetResultUID(&resultDetails, clients, &chaosDetails); err != nil {
		log.WithContext(ctx).Errorf("Unable to set the result uid, err: %v", err)
		result.RecordAfterFailure(&chaosDetails, &resultDetails, err, clients, &eventsDetails)
		span.SetStatus(codes.Error, "Unable to set the result uid")
		span.RecordError(err)
		return
	}

	// generating the event in chaosresult to mark the verdict as awaited
	msg := "experiment: " + experimentsDetails.ExperimentName + ", Result: Awaited"
	types.SetResultEventAttributes(&eventsDetails, types.AwaitedVerdict, msg, "Normal", &resultDetails)
	if err := events.GenerateEvents(&eventsDetails, clients, &chaosDetails, "ChaosResult"); err != nil {
		log.WithContext(ctx).Errorf("failed to create %v event inside chaosresult", types.AwaitedVerdict)
	}

	//DISPLAY THE APP INFORMATION
	log.WithContext(ctx).InfoWithValues("The application information is as follows", logrus.Fields{
		"Targets":        common.GetAppDetailsForLogging(chaosDetails.AppDetail),
		"Chaos Duration": experimentsDetails.ChaosDuration,
	})

	// Calling AbortWatcher go routine, it will continuously watch for the abort signal and generate the required events and result
	go common.AbortWatcher(experimentsDetails.ExperimentName, clients, &resultDetails, &chaosDetails, &eventsDetails)

	//PRE-CHAOS APPLICATION STATUS CHECK
	if chaosDetails.DefaultHealthCheck {
		log.WithContext(ctx).Info("[Status]: Verify that the AUT (Application Under Test) is running (pre-chaos)")
		if err := status.AUTStatusCheck(clients, &chaosDetails); err != nil {
			log.WithContext(ctx).Errorf("Application status check failed, err: %v", err)
			types.SetEngineEventAttributes(&eventsDetails, types.PreChaosCheck, "AUT: Not Running", "Warning", &chaosDetails)
			if eventErr := events.GenerateEvents(&eventsDetails, clients, &chaosDetails, "ChaosEngine"); eventErr != nil {
				log.WithContext(ctx).Errorf("failed to create %v event inside chaosengine", types.PreChaosCheck)
			}
			result.RecordAfterFailure(&chaosDetails, &resultDetails, err, clients, &eventsDetails)
			span.SetStatus(codes.Error, "Application status check failed")
			span.RecordError(err)
			return
		}
	}

	if experimentsDetails.EngineName != "" {
		// marking AUT as running, as we already checked the status of application under test
		msg := common.GetStatusMessage(chaosDetails.DefaultHealthCheck, "AUT: Running", "")

		// run the probes in the pre-chaos check
		if len(resultDetails.ProbeDetails) != 0 {

			if err := probe.RunProbes(ctx, &chaosDetails, clients, &resultDetails, "PreChaos", &eventsDetails); err != nil {
				log.WithContext(ctx).Errorf("Probe Failed, err: %v", err)
				msg = common.GetStatusMessage(chaosDetails.DefaultHealthCheck, "AUT: Running", "Unsuccessful")
				types.SetEngineEventAttributes(&eventsDetails, types.PreChaosCheck, msg, "Warning", &chaosDetails)
				if eventErr := events.GenerateEvents(&eventsDetails, clients, &chaosDetails, "ChaosEngine"); eventErr != nil {
					log.WithContext(ctx).Errorf("failed to create %v event inside chaosengine", types.PreChaosCheck)
				}
				result.RecordAfterFailure(&chaosDetails, &resultDetails, err, clients, &eventsDetails)
				span.SetStatus(codes.Error, "Probe Failed")
				span.RecordError(err)
				return
			}
			msg = common.GetStatusMessage(chaosDetails.DefaultHealthCheck, "AUT: Running", "Successful")
		}
		// generating the events for the pre-chaos check
		types.SetEngineEventAttributes(&eventsDetails, types.PreChaosCheck, msg, "Normal", &chaosDetails)
		events.GenerateEvents(&eventsDetails, clients, &chaosDetails, "ChaosEngine")
	}

	chaosDetails.Phase = types.ChaosInjectPhase
	if err := litmusLIB.PreparePodDelete(ctx, &experimentsDetails, clients, &resultDetails, &eventsDetails, &chaosDetails); err != nil {
		log.WithContext(ctx).Errorf("Chaos injection failed, err: %v", err)
		result.RecordAfterFailure(&chaosDetails, &resultDetails, err, clients, &eventsDetails)
		span.SetStatus(codes.Error, "Chaos injection failed")
		span.RecordError(err)
		return
	}

	log.WithContext(ctx).Infof("[Confirmation]: %v chaos has been injected successfully", experimentsDetails.ExperimentName)
	resultDetails.Verdict = v1alpha1.ResultVerdictPassed
	chaosDetails.Phase = types.PostChaosPhase

	//POST-CHAOS APPLICATION STATUS CHECK
	if chaosDetails.DefaultHealthCheck {
		log.WithContext(ctx).Info("[Status]: Verify that the AUT (Application Under Test) is running (post-chaos)")
		if err := status.AUTStatusCheck(clients, &chaosDetails); err != nil {
			log.WithContext(ctx).Errorf("Application status check failed, err: %v", err)
			types.SetEngineEventAttributes(&eventsDetails, types.PostChaosCheck, "AUT: Not Running", "Warning", &chaosDetails)
			events.GenerateEvents(&eventsDetails, clients, &chaosDetails, "ChaosEngine")
			result.RecordAfterFailure(&chaosDetails, &resultDetails, err, clients, &eventsDetails)
			span.SetStatus(codes.Error, "Application status check failed")
			span.RecordError(err)
			return
		}
	}

	if experimentsDetails.EngineName != "" {
		// marking AUT as running, as we already checked the status of application under test
		msg := common.GetStatusMessage(chaosDetails.DefaultHealthCheck, "AUT: Running", "")

		// run the probes in the post-chaos check
		if len(resultDetails.ProbeDetails) != 0 {
			if err := probe.RunProbes(ctx, &chaosDetails, clients, &resultDetails, "PostChaos", &eventsDetails); err != nil {
				log.WithContext(ctx).Errorf("Probes Failed, err: %v", err)
				msg = common.GetStatusMessage(chaosDetails.DefaultHealthCheck, "AUT: Running", "Unsuccessful")
				types.SetEngineEventAttributes(&eventsDetails, types.PostChaosCheck, msg, "Warning", &chaosDetails)
				if eventErr := events.GenerateEvents(&eventsDetails, clients, &chaosDetails, "ChaosEngine"); eventErr != nil {
					log.WithContext(ctx).Errorf("failed to create %v event inside chaosengine", types.PostChaosCheck)
				}
				result.RecordAfterFailure(&chaosDetails, &resultDetails, err, clients, &eventsDetails)
				span.SetStatus(codes.Error, "Probes Failed")
				span.RecordError(err)
				return
			}
			msg = common.GetStatusMessage(chaosDetails.DefaultHealthCheck, "AUT: Running", "Successful")
		}

		// generating post chaos event
		types.SetEngineEventAttributes(&eventsDetails, types.PostChaosCheck, msg, "Normal", &chaosDetails)
		events.GenerateEvents(&eventsDetails, clients, &chaosDetails, "ChaosEngine")
	}

	//Updating the chaosResult in the end of experiment
	log.WithContext(ctx).Infof("[The End]: Updating the chaos result of %v experiment (EOT)", experimentsDetails.ExperimentName)
	if err := result.ChaosResult(&chaosDetails, clients, &resultDetails, "EOT"); err != nil {
		log.WithContext(ctx).Errorf("Unable to update the chaosresult, err: %v", err)
		result.RecordAfterFailure(&chaosDetails, &resultDetails, err, clients, &eventsDetails)
		span.SetStatus(codes.Error, "Unable to update the chaosresult")
		span.RecordError(err)
		return
	}

	// generating the event in chaosresult to mark the verdict as pass/fail
	msg = "experiment: " + experimentsDetails.ExperimentName + ", Result: " + string(resultDetails.Verdict)
	reason, eventType := types.GetChaosResultVerdictEvent(resultDetails.Verdict)
	types.SetResultEventAttributes(&eventsDetails, reason, msg, eventType, &resultDetails)
	events.GenerateEvents(&eventsDetails, clients, &chaosDetails, "ChaosResult")

	if experimentsDetails.EngineName != "" {
		msg := experimentsDetails.ExperimentName + " experiment has been " + string(resultDetails.Verdict) + "ed"
		types.SetEngineEventAttributes(&eventsDetails, types.Summary, msg, "Normal", &chaosDetails)
		events.GenerateEvents(&eventsDetails, clients, &chaosDetails, "ChaosEngine")
	}
}
