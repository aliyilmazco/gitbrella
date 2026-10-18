package moveout

import (
	"context"
	"testing"
	"time"

	"github.com/aliyilmazco/gitbrella/internal/icloud"
)

// An upload that finishes within the wait plans the move; one that does not
// is still refused.
func TestPlanWaitsForTheUpload(t *testing.T) {
	for name, tc := range map[string]struct {
		pending int           // probes that still answer "uploading"
		wait    time.Duration // the upload wait; a finishing one never needs all of it
		want    string
	}{
		"finishes": {pending: 3, wait: time.Minute, want: ""},
		"too slow": {pending: 1 << 30, wait: 2 * time.Second, want: RefuseICloudUploading},
	} {
		t.Run(name, func(t *testing.T) {
			l := newLab(t)
			n := 0
			l.opts.Probe = func(string) (icloud.Status, error) {
				n++
				return icloud.Status{Known: true, Ubiquitous: true, Uploaded: n > tc.pending}, nil
			}
			p, err := PlanWaiting(context.Background(), l.g, l.opts, tc.wait, 10*time.Millisecond)
			if err != nil {
				t.Fatal(err)
			}
			if got := codes(p.Refusals); got != tc.want {
				t.Fatalf("refusals %q, want %q", got, tc.want)
			}
		})
	}
}
