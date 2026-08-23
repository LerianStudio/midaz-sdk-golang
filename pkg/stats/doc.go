// Package stats provides lightweight in-process counters used by the
// SDK's demo generator and any consumer code that wants a basic TPS
// (transactions-per-second) view without pulling in a full metrics
// library.
//
// The package stays deliberately minimal: a [Counter] tracks
// successful events and exposes start time + count for derived rates.
// Concurrency-safe via sync/atomic. No allocations on the hot path.
//
// # Public surface
//
//   - [Counter] — atomic success counter with start-time anchor.
//   - [NewCounter] — constructor (sets start time to now).
//   - [Counter.RecordSuccess] — atomic increment.
//   - Derived helpers expose elapsed time and TPS.
//
// # Quickstart
//
//	c := stats.NewCounter()
//	for /* work loop */ {
//	    if err := doStuff(); err == nil {
//	        c.RecordSuccess()
//	    }
//	}
//	fmt.Printf("TPS: %.2f\n", c.TPS())
//
// # When to use
//
//   - Demo programs and integration tests where you want a rate
//     view without wiring full observability.
//
// # When NOT to use
//
//   - Production observability. Use
//     [github.com/LerianStudio/midaz-sdk-golang/v5/pkg/observability]
//     instead — it emits OpenTelemetry metrics that downstream
//     dashboards already understand.
//
// # See also
//
//   - [github.com/LerianStudio/midaz-sdk-golang/v5/pkg/observability]
//   - examples/mass-demo-generator
package stats
