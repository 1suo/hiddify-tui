package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/1suo/hiddify-tui/internal/control"
)

func ProfileList(ctx context.Context, daemon control.ProfileReader, jsonOutput bool, stdout, stderr io.Writer) int {
	profiles, err := daemon.ListProfiles(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "profile list: %v\n", err)
		return ExitUnavailable
	}
	if jsonOutput {
		if err := json.NewEncoder(stdout).Encode(struct {
			SchemaVersion uint32            `json:"schema_version"`
			Profiles      []control.Profile `json:"profiles"`
		}{SchemaVersion: 1, Profiles: profiles}); err != nil {
			fmt.Fprintf(stderr, "profile list: %v\n", err)
			return ExitRejected
		}
		return ExitOK
	}
	writer := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "ACTIVE\tID\tNAME\tTYPE\tSOURCE")
	for _, profile := range profiles {
		active := ""
		if profile.Active {
			active = "*"
		}
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n", active, profile.ID, profile.Name, profile.Kind, valueOr(profile.RedactedURL, "local"))
	}
	if err := writer.Flush(); err != nil {
		fmt.Fprintf(stderr, "profile list: %v\n", err)
		return ExitRejected
	}
	return ExitOK
}

func ProfileShow(ctx context.Context, daemon control.ProfileReader, id string, jsonOutput bool, stdout, stderr io.Writer) int {
	profile, err := daemon.GetProfile(ctx, id)
	if err != nil {
		fmt.Fprintf(stderr, "profile show: %v\n", err)
		return ExitUnavailable
	}
	if jsonOutput {
		if err := json.NewEncoder(stdout).Encode(struct {
			SchemaVersion uint32          `json:"schema_version"`
			Profile       control.Profile `json:"profile"`
		}{SchemaVersion: 1, Profile: profile}); err != nil {
			fmt.Fprintf(stderr, "profile show: %v\n", err)
			return ExitRejected
		}
		return ExitOK
	}
	fmt.Fprintf(stdout, "ID: %s\nName: %s\nType: %s\nActive: %t\nSource: %s\n", profile.ID, profile.Name, profile.Kind, profile.Active, valueOr(profile.RedactedURL, "local"))
	return ExitOK
}
