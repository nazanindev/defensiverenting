// Package mcpserver adapts the drafting toolbelt to MCP, so Claude Code (or any
// MCP client) can drive the research→draft loop. It is a thin front-end: all the
// logic — including the verbatim-citation guardrail — lives in internal/drafting,
// which the production worker (cmd/draft) drives through the same methods.
package mcpserver

import (
	"context"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nazanindev/defensiverenting/internal/drafting"
	"github.com/nazanindev/defensiverenting/internal/store"
)

// New builds an MCP server exposing the drafting toolbelt.
func New(db store.Store) *mcp.Server {
	tb := drafting.New(db)
	srv := mcp.NewServer(&mcp.Implementation{Name: "defensiverenting", Version: "v0.1.0"}, nil)

	mcp.AddTool(srv, &mcp.Tool{
		Name: "find_sources",
		Description: "List ranked, vetted authoritative primary sources (statutes, ordinances, " +
			"government guidance, legal-aid orgs) for a city, to seed research. Start here.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in drafting.FindSourcesInput) (*mcp.CallToolResult, drafting.FindSourcesOutput, error) {
		return result(tb.FindSources(ctx, in))
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "fetch_source",
		Description: "Fetch a source URL and return its readable text. You MUST fetch a source " +
			"through this tool before citing it: save_draft_playbook only accepts quotes that " +
			"appear verbatim in text returned here.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in drafting.FetchSourceInput) (*mcp.CallToolResult, drafting.FetchSourceOutput, error) {
		return result(tb.FetchSource(ctx, in))
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name: "save_draft_playbook",
		Description: "Save a DRAFT page (statements + citations) for the author to verify and " +
			"publish. Every citation's quote must be a verbatim line from a source you fetched " +
			"via fetch_source, or the save is rejected. This never publishes anything.\n\n" +
			"topic_slug and page_kind are two different things and are chosen separately.\n" +
			"  topic_slug is WHAT THE PAGE IS ABOUT. It comes from list_topics and cannot be invented.\n" +
			"  page_kind is HOW IT IS LAID OUT: \"playbook\" renders statements as a numbered " +
			"argument, \"directory\" renders each statement as an entry in a list of organisations. " +
			"Omit page_kind for playbook.\n\n" +
			"One pairing is enforced: topic_slug=\"resource-directory\" (display name \"Local Help\") " +
			"must use page_kind=\"directory\". That topic lists organisations rather than explaining " +
			"law, so the playbook layout would render them as legal steps. The reverse is allowed — " +
			"any topic may use the directory layout when the honest answer to it is a list of places " +
			"to go.\n\n" +
			"Drafting Local Help: one organisation per statement, each citing that organisation's " +
			"own page (kind \"nonprofit\") or its government program page (kind \"gov_guidance\"). " +
			"Never cite an aggregator that is summarising other organisations; find the org's own " +
			"URL or leave it out. A city needs this page once, so do not repeat local help at the " +
			"foot of every playbook.\n\n" +
			"Two rules that reject a save outright: a citation with kind \"statute\" must carry a " +
			"locator naming a provision (\"§ 15B\", \"RCW 59.18.060\"), not a date or a document " +
			"title; and every non-editorial citation needs a verbatim quote, because a page whose " +
			"citations have no quote cannot be published at all.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in drafting.SaveDraftInput) (*mcp.CallToolResult, drafting.SaveDraftOutput, error) {
		return result(tb.SaveDraft(ctx, in))
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_jurisdictions",
		Description: "List the cities known to the platform (slug + name), for resolving a city.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, drafting.ListJurisdictionsOutput, error) {
		return result(tb.ListJurisdictions(ctx))
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_topics",
		Description: "List topics that already have a published playbook for a city, so you don't duplicate existing coverage.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in drafting.ListTopicsInput) (*mcp.CallToolResult, drafting.ListTopicsOutput, error) {
		return result(tb.ListTopics(ctx, in))
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_playbook",
		Description: "Fetch an existing published playbook for a city+topic (to review current coverage). Returns not-found if none exists.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in drafting.GetPlaybookInput) (*mcp.CallToolResult, drafting.GetPlaybookOutput, error) {
		return result(tb.GetPlaybook(ctx, in))
	})

	return srv
}

// result maps a toolbelt method's (Out, error) return onto MCP's tool-result
// shape: a *RejectionError becomes an agent-visible IsError result the model can
// read and correct; any other error propagates as a protocol error.
func result[Out any](out Out, err error) (*mcp.CallToolResult, Out, error) {
	var re *drafting.RejectionError
	if errors.As(err, &re) {
		var zero Out
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{&mcp.TextContent{Text: re.Msg}},
		}, zero, nil
	}
	if err != nil {
		var zero Out
		return nil, zero, err
	}
	return nil, out, nil
}
