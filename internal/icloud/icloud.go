// Package icloud asks the system whether iCloud Drive still has uploads to
// make for a path. It reads metadata only; nothing is downloaded.
package icloud

// Status is what iCloud reports for one item.
type Status struct {
	Known      bool // the system answered (macOS)
	Ubiquitous bool // iCloud syncs the item
	Uploaded   bool
	Uploading  bool
}

// Pending reports whether iCloud has not finished sending the item.
func (s Status) Pending() bool { return s.Known && s.Ubiquitous && (!s.Uploaded || s.Uploading) }
