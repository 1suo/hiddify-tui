package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/1suo/hiddify-tui/internal/client"
)

// OutboundList prints the current outbound groups.
func OutboundList(ctx context.Context, core client.Client, jsonOutput bool, stdout, stderr io.Writer) int {
	groups, err := core.OutboundGroups(ctx)
	if err != nil {
		WriteError(stderr, "outbound list", err)
		return ExitUnavailable
	}
	if jsonOutput {
		if err := writeJSON(stdout, groups); err != nil {
			return ExitRejected
		}
		return ExitOK
	}
	for _, group := range groups {
		fmt.Fprintf(stdout, "%s [%s]\n", group.Tag, group.Type)
		for _, item := range group.Items {
			selected := " "
			if item.Selected {
				selected = "*"
			}
			delay := "-"
			if item.DelayMillis > 0 {
				delay = fmt.Sprintf("%d ms", item.DelayMillis)
			}
			fmt.Fprintf(stdout, "  %s %s  %s  %s\n", selected, item.Tag, item.Type, delay)
		}
	}
	return ExitOK
}

// OutboundSelect selects an outbound within a group.
func OutboundSelect(ctx context.Context, core client.Client, group, outbound string, stdout, stderr io.Writer) int {
	if err := core.SelectOutbound(ctx, group, outbound); err != nil {
		WriteError(stderr, "outbound select", err)
		return ExitRejected
	}
	fmt.Fprintf(stdout, "selected %s in %s\n", outbound, group)
	return ExitOK
}

// OutboundTest runs a URL test on one outbound.
func OutboundTest(ctx context.Context, core client.Client, tag string, stdout, stderr io.Writer) int {
	if err := core.TestOutbound(ctx, tag); err != nil {
		WriteError(stderr, "outbound test", err)
		return ExitRejected
	}
	fmt.Fprintf(stdout, "tested %s\n", tag)
	return ExitOK
}
