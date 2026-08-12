package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/1suo/hiddify-tui/internal/control"
)

func OutboundList(ctx context.Context, daemon control.OutboundOperator, jsonOutput bool, stdout, stderr io.Writer) int {
	groups, err := daemon.ListOutboundGroups(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "outbound list: %v\n", err)
		return ExitUnavailable
	}
	if jsonOutput {
		if err := json.NewEncoder(stdout).Encode(struct {
			SchemaVersion uint32                  `json:"schema_version"`
			Groups        []control.OutboundGroup `json:"groups"`
		}{SchemaVersion: 1, Groups: groups}); err != nil {
			fmt.Fprintf(stderr, "outbound list: %v\n", err)
			return ExitRejected
		}
		return ExitOK
	}
	writer := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "GROUP\tSELECTED\tTAG\tPROTOCOL\tDELAY")
	for _, group := range groups {
		for _, outbound := range group.Outbounds {
			selected := ""
			if group.SelectedOutboundID == outbound.ID {
				selected = "*"
			}
			delay := "-"
			if outbound.DelayMillis > 0 {
				delay = fmt.Sprintf("%d ms", outbound.DelayMillis)
			}
			fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n", group.Name, selected, outbound.Tag, outbound.Protocol, delay)
		}
	}
	if err := writer.Flush(); err != nil {
		fmt.Fprintf(stderr, "outbound list: %v\n", err)
		return ExitRejected
	}
	return ExitOK
}

func OutboundSelect(ctx context.Context, daemon control.OutboundOperator, groupID, outboundID string, stdout, stderr io.Writer) int {
	if err := daemon.SelectOutbound(ctx, groupID, outboundID); err != nil {
		fmt.Fprintf(stderr, "outbound select: %v\n", err)
		return ExitRejected
	}
	fmt.Fprintf(stdout, "Outbound selected: %s\n", outboundID)
	return ExitOK
}

func OutboundTest(ctx context.Context, daemon control.OutboundOperator, scope control.TestScope, stdout, stderr io.Writer) int {
	if err := daemon.TestOutbounds(ctx, scope); err != nil {
		fmt.Fprintf(stderr, "outbound test: %v\n", err)
		return ExitRejected
	}
	fmt.Fprintln(stdout, "Outbound test requested")
	return ExitOK
}
