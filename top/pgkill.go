// Stuff that allows to cancel Postgres queries and terminate backends.

package top

import (
	"fmt"
	"github.com/jroimartin/gocui"
	"github.com/lesovsky/pgcenter/internal/postgres"
	"github.com/lesovsky/pgcenter/internal/query"
	"strconv"
	"strings"
)

const (
	groupActive int = 1 << iota
	groupIdle
	groupIdleXact
	groupWaiting
	groupOthers
)

// killSingle sends cancel or terminate signal to a single Postgres backend.
func killSingle(db *postgres.DB, mode string, buf string) error {
	if mode != "cancel" && mode != "terminate" {
		return fmt.Errorf("invalid input")
	}

	var q, answer string

	switch mode {
	case "cancel":
		q = query.ExecCancelQuery
		answer = strings.TrimPrefix(buf, dialogPrompts[dialogCancelQuery])
	case "terminate":
		q = query.ExecTerminateBackend
		answer = strings.TrimPrefix(buf, dialogPrompts[dialogTerminateBackend])
	}
	answer = strings.TrimSuffix(answer, "\n")

	pid, err := strconv.Atoi(answer)
	if err != nil {
		return err
	}

	_, err = db.Exec(q, pid)
	if err != nil {
		return err
	}

	return nil
}

// killGroup sends cancel or terminate signal to group of Postgres backends.
func killGroup(app *app, mode string) (string, error) {
	if app.config.view.Name != "activity" {
		return "Terminate or cancel backend allowed in pg_stat_activity.", nil
	}

	mask := app.config.procMask

	if mask == 0 {
		return "Do nothing. The mask is empty.", nil
	}

	if mode != "cancel" && mode != "terminate" {
		return "Do nothing. Unknown mode (not cancel, nor terminate).", nil
	}

	var (
		template, q               string
		signalled, signalledTotal int64
	)

	// Select signal function: pg_cancel_backend or pg_terminate_backend.
	switch mode {
	case "cancel":
		template = query.ExecCancelQueryGroup
	case "terminate":
		template = query.ExecTerminateBackendGroup
	}

	// states defines SQL expression conditions necessary for selecting group of target processes.
	var states = map[int]string{
		groupIdle:     "state = 'idle'",
		groupIdleXact: "state IN ('idle in transaction (aborted)', 'idle in transaction')",
		groupActive:   "state = 'active'",
		groupWaiting:  "wait_event IS NOT NULL OR wait_event_type IS NOT NULL",
		groupOthers:   "state IN ('fastpath function call', 'disabled')",
	}

	// Walk through the states, if state is in the mask then send signal to that group of process.
	for state, part := range states {
		if (mask & state) != 0 {
			app.config.queryOptions.BackendState = part
			if state == groupWaiting && app.postgresProps.VersionNum < 90600 {
				app.config.queryOptions.BackendState = "waiting"
			}
			q, _ = query.Format(template, app.config.queryOptions)
			err := app.db.QueryRow(q).Scan(&signalled)
			if err != nil {
				return "", err
			}

			signalledTotal += signalled
		}
	}

	var msg string
	switch mode {
	case "cancel":
		msg = "Cancelled " + strconv.FormatInt(signalledTotal, 10) + " queries."
	case "terminate":
		msg = "Terminated " + strconv.FormatInt(signalledTotal, 10) + " backends."
	}

	return msg, nil
}

// setProcMask
func setProcMask(_ *gocui.Gui, buf string, config *config) {
	answer := strings.TrimPrefix(buf, dialogPrompts[dialogSetMask])
	answer = strings.TrimSuffix(answer, "\n")

	// Reset existing mask.
	config.procMask = 0

	for _, ch := range answer {
		switch string(ch) {
		case "i":
			config.procMask |= groupIdle
		case "x":
			config.procMask |= groupIdleXact
		case "a":
			config.procMask |= groupActive
		case "w":
			config.procMask |= groupWaiting
		case "o":
			config.procMask |= groupOthers
		}
	}

	showProcMask(config.procMask)
}

// showProcMask
func showProcMask(mask int) func(g *gocui.Gui, _ *gocui.View) error {
	return func(g *gocui.Gui, _ *gocui.View) error {
		ct := "Mask: "
		if mask == 0 {
			ct += "empty "
		}
		if (mask & groupIdle) != 0 {
			ct += "idle "
		}
		if (mask & groupIdleXact) != 0 {
			ct += "idle_xact "
		}
		if (mask & groupActive) != 0 {
			ct += "active "
		}
		if (mask & groupWaiting) != 0 {
			ct += "waiting "
		}
		if (mask & groupOthers) != 0 {
			ct += "others "
		}

		printCmdline(g, ct)

		return nil
	}
}
