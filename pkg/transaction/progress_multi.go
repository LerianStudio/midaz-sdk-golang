package transaction

import (
	"sync"

	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

// MultiProgressController manages multiple progress bars: overall, success, failed.
type MultiProgressController struct {
	mu      sync.Mutex
	p       *mpb.Progress
	overall *mpb.Bar
	success *mpb.Bar
	failed  *mpb.Bar
	total   int
}

// NewMultiProgressController creates a multi-bar controller with overall/success/failed bars.
func NewMultiProgressController(total int, description string) *MultiProgressController {
	p := mpb.New(mpb.WithWidth(64))

	overall := p.AddBar(int64(total),
		mpb.PrependDecorators(
			decor.Name(description+" ", decor.WCSyncWidth),
			decor.CountersNoUnit("%d/%d", decor.WCSyncWidth),
		),
		mpb.AppendDecorators(
			decor.EwmaETA(decor.ET_STYLE_GO, 60),
		),
	)

	success := p.AddBar(int64(total),
		mpb.PrependDecorators(
			decor.Name("success ", decor.WCSyncWidth),
			decor.CountersNoUnit("%d/%d", decor.WCSyncWidth),
		),
	)

	failed := p.AddBar(int64(total),
		mpb.PrependDecorators(
			decor.Name("failed  ", decor.WCSyncWidth),
			decor.CountersNoUnit("%d/%d", decor.WCSyncWidth),
		),
	)

	return &MultiProgressController{
		p:       p,
		overall: overall,
		success: success,
		failed:  failed,
		total:   total,
	}
}

// OnProgressCallback returns a BatchOptions.OnProgress-compatible function that updates all bars.
func (mpc *MultiProgressController) OnProgressCallback() func(completed, total int, result BatchResult) {
	return func(_completed, _total int, result BatchResult) {
		mpc.mu.Lock()
		defer mpc.mu.Unlock()
		mpc.overall.Increment()
		if result.Error == nil {
			mpc.success.Increment()
		} else {
			mpc.failed.Increment()
		}
	}
}

// Wait waits for all bars to complete rendering.
func (mpc *MultiProgressController) Wait() {
	mpc.p.Wait()
}
