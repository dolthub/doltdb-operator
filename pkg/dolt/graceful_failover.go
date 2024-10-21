package dolt

import (
	"context"
	"fmt"
	"log"

	doltclusterctl "github.com/dolthub/doltclusterctl"
)

func PerformGracefulFailover(ctx context.Context, cfg *doltclusterctl.Config, cluster KubernetesClusterContext) error {
	dbstates := doltclusterctl.LoadDBStates(ctx, cfg, cluster)

	var errStates []doltclusterctl.DBState
	for _, state := range dbstates {
		if state.Err != nil {
			errStates = append(errStates, state)
		}
	}

	if len(errStates) > 0 && cfg.MinCaughtUpStandbys == -1 {
		// If we need to catch up all standbys, but we
		// can't currently reach one of the standbys,
		// something is wrong. We do not go forward with
		// the attempt.
		return fmt.Errorf("error loading role and epoch for pod %s: %w", errStates[0].Instance.Name(), errStates[0].Err)
	}

	numStandbys := len(dbstates) - 1
	numReachableStandbys := len(dbstates) - len(errStates) - 1

	if cfg.MinCaughtUpStandbys != -1 {
		if numStandbys < cfg.MinCaughtUpStandbys {
			return fmt.Errorf("Invalid min-caughtup-standbys of %d provided. Only %d pods are in the cluster, so only %d standbys can ever be caught up.", cfg.MinCaughtUpStandbys, len(dbstates), len(dbstates)-1)
		}
		if numReachableStandbys < cfg.MinCaughtUpStandbys {
			return fmt.Errorf("could not reach enough standbys to catch up %d. Out of %d pods, %d were unreachable. For example, contacting pod %s resulted in error: %w", cfg.MinCaughtUpStandbys, len(dbstates), len(errStates), errStates[0].Instance.Name(), errStates[0].Err)
		}
	}

	// Find current primary across the pods.
	currentprimary, highestepoch, err := doltclusterctl.CurrentPrimaryAndEpoch(dbstates)
	if err != nil {
		return fmt.Errorf("cannot perform graceful failover: %w", err)
	}

	oldPrimary := dbstates[currentprimary].Instance
	nextepoch := highestepoch + 1

	if cfg.MinCaughtUpStandbys != -1 && !doltclusterctl.VersionSupportsTransitionToStandby(dbstates[currentprimary].Version) {
		return fmt.Errorf("Cannot perform gracefulfailover with min-caughtup-standbys of %d. The version of Dolt on the current primary (%s on pod %s) does not support dolt_cluster_transition_to_standby.", cfg.MinCaughtUpStandbys, dbstates[currentprimary].Version, oldPrimary.Name())
	}

	log.Printf("failing over from %s", oldPrimary.Name())

	for _, state := range dbstates {
		err := state.Instance.MarkRoleStandby(ctx)
		if err != nil {
			return err
		}
	}

	log.Printf("labeled all pods standby")

	var newPrimary doltclusterctl.Instance

	if cfg.MinCaughtUpStandbys == -1 {
		nextprimary := (currentprimary + 1) % cluster.NumReplicas()
		newPrimary = dbstates[nextprimary].Instance

		err = doltclusterctl.CallAssumeRole(ctx, cfg, oldPrimary, "standby", nextepoch)
		if err != nil {
			log.Printf("failed to transition primary to standby. labeling old primary as primary.")
			err = oldPrimary.MarkRolePrimary(ctx)
			if err != nil {
				log.Printf("ERROR: failed to label old primary as primary.")
				log.Printf("\t%v", err)
				log.Printf("dolt-rw endpoint will be broken. You need to run applyprimarylabels.")
			}
			return fmt.Errorf("error calling dolt_assume_cluster_role standby on %s: %w", oldPrimary.Name(), err)
		}
		log.Printf("called dolt_assume_cluster_role standby on %s", oldPrimary.Name())
	} else {
		nextprimary, err := doltclusterctl.CallTransitionToStandby(ctx, cfg, oldPrimary, nextepoch, dbstates)
		if err != nil {
			log.Printf("failed to transition primary to standby. labeling old primary as primary.")
			err = oldPrimary.MarkRolePrimary(ctx)
			if err != nil {
				log.Printf("ERROR: failed to label old primary as primary.")
				log.Printf("\t%v", err)
				log.Printf("dolt-rw endpoint will be broken. You need to run applyprimarylabels.")
			}
			return fmt.Errorf("error calling dolt_cluster_transition_to_standby on %s: %w", oldPrimary.Name(), err)
		}
		log.Printf("called dolt_cluster_transition_to_standby on %s", oldPrimary.Name())
		newPrimary = dbstates[nextprimary].Instance
	}

	log.Printf("failing over to %s", newPrimary.Name())

	err = doltclusterctl.CallAssumeRole(ctx, cfg, newPrimary, "primary", nextepoch)
	if err != nil {
		return err
	}

	log.Printf("called dolt_assume_cluster_role primary on %s", newPrimary.Name())

	err = newPrimary.MarkRolePrimary(ctx)
	if err != nil {
		return err
	}

	log.Printf("added primary label to %s", newPrimary.Name())

	return nil
}
