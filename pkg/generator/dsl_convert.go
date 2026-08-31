package generator

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"

	"github.com/LerianStudio/midaz-sdk-golang/v6/models"
)

var (
	dslSendRe      = regexp.MustCompile(`send\s*\[\s*(\S+)\s+(\d+)\s*\]`)
	dslDistAssetRe = regexp.MustCompile(`distribute\s*\[\s*(\S+)\s+\d+\s*\]`)
	dslSourceRe    = regexp.MustCompile(`source\s*=\s*(\S+)`)
	dslDestRe      = regexp.MustCompile(`(\d+)%\s+to\s+(\S+)`)
)

// dslTemplateToInput converts the fixed-grammar DSL templates emitted by
// pkg/data (send [asset amount] (source = X) distribute [...] (destination = {
// pct% to Y ... })) into a structured models.CreateTransactionInput for
// CreateJSON. Destination percentages map to distribution Shares, so no
// amount math is done here.
//
// ponytail: handles only the pkg/data grammar (single source at the full
// amount, percentage destinations, single asset). Multi-asset templates (send
// asset != distribute asset, e.g. an FX exchange) are rejected rather than
// silently mishandled — the converter maps the send asset onto every leg, so a
// second distribute asset would be dropped and produce a balanced self-transfer
// that moves nothing. The general DSL→/json helper is Epic 5.5; this replaces
// the dropped CreateTransactionWithDSLFile path without reviving the wire /dsl
// endpoint.
func dslTemplateToInput(tmpl string) (*models.CreateTransactionInput, error) {
	send := dslSendRe.FindStringSubmatch(tmpl)
	if send == nil {
		return nil, errors.New("dsl: no 'send [asset amount]' clause")
	}

	asset, amount := send[1], send[2]

	// The converter only carries the send asset onto every leg, so a distribute
	// clause naming a different asset would be silently dropped. Reject it.
	if dist := dslDistAssetRe.FindStringSubmatch(tmpl); dist != nil && dist[1] != asset {
		return nil, fmt.Errorf("dsl: multi-asset templates are not supported (send asset %q != distribute asset %q)", asset, dist[1])
	}

	src := dslSourceRe.FindStringSubmatch(tmpl)
	if src == nil {
		return nil, errors.New("dsl: no 'source = ' clause")
	}

	dests := dslDestRe.FindAllStringSubmatch(tmpl, -1)
	if len(dests) == 0 {
		return nil, errors.New("dsl: no 'pct% to alias' destinations")
	}

	to := make([]models.FromToInput, 0, len(dests))

	for _, d := range dests {
		pct, err := strconv.ParseInt(d[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("dsl: invalid percentage %q: %w", d[1], err)
		}

		to = append(to, models.FromToInput{
			AccountAlias: d[2],
			Share:        &models.Share{Percentage: pct},
		})
	}

	return &models.CreateTransactionInput{
		Send: &models.SendInput{
			Asset: asset,
			Value: amount,
			Source: &models.SourceInput{
				From: []models.FromToInput{
					{AccountAlias: src[1], Amount: models.AmountInput{Asset: asset, Value: amount}},
				},
			},
			Distribute: &models.DistributeInput{To: to},
		},
	}, nil
}
