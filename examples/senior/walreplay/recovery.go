package walreplay

import (
	"errors"
	"fmt"

	"td-homework/examples/senior/matchingengine"
)

// Processor enforces the command-log ordering used by this example:
// validate envelope -> append durable command -> apply deterministic state.
type Processor struct {
	Engine *matchingengine.Engine
	WAL    *WAL
}

func (p *Processor) Apply(command matchingengine.Command) ([]matchingengine.Event, error) {
	if p.Engine == nil || p.WAL == nil {
		return nil, errors.New("processor requires engine and WAL")
	}
	if err := matchingengine.ValidateCommand(command, p.Engine.LastSequence()+1); err != nil {
		return nil, err
	}
	if err := p.WAL.Append(command); err != nil {
		return nil, fmt.Errorf("append WAL: %w", err)
	}
	events, err := p.Engine.Apply(command)
	if err != nil {
		// A validated deterministic command failing here is an invariant breach.
		// Stop the partition; do not accept a later command.
		return nil, fmt.Errorf("apply durable command: %w", err)
	}
	return events, nil
}

// Recover loads a snapshot and replays the WAL suffix. It returns events
// regenerated after the snapshot; a production publisher resumes from its own
// durable publication watermark and must remain idempotent.
func Recover(snapshotPath, walPath string, repairTornTail bool) (*matchingengine.Engine, []matchingengine.Event, error) {
	snapshot, exists, err := ReadSnapshot(snapshotPath)
	if err != nil {
		return nil, nil, err
	}
	engine := matchingengine.New()
	if exists {
		engine, err = matchingengine.FromSnapshot(snapshot)
		if err != nil {
			return nil, nil, fmt.Errorf("restore snapshot: %w", err)
		}
	}

	commands, validBytes, scanErr := Scan(walPath)
	var torn *TornTailError
	if errors.As(scanErr, &torn) {
		if !repairTornTail {
			return nil, nil, scanErr
		}
		if err = RepairTornTail(walPath, validBytes); err != nil {
			return nil, nil, fmt.Errorf("repair torn WAL tail: %w", err)
		}
	} else if scanErr != nil {
		return nil, nil, scanErr
	}

	var replayed []matchingengine.Event
	for _, command := range commands {
		if command.Sequence <= engine.LastSequence() {
			continue
		}
		if command.Sequence != engine.LastSequence()+1 {
			return nil, nil, fmt.Errorf(
				"WAL sequence gap: got %d after %d",
				command.Sequence,
				engine.LastSequence(),
			)
		}
		events, applyErr := engine.Apply(command)
		if applyErr != nil {
			return nil, nil, fmt.Errorf("replay sequence %d: %w", command.Sequence, applyErr)
		}
		replayed = append(replayed, events...)
	}
	return engine, replayed, nil
}
