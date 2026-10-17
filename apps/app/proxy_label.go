package app

import (
	"fmt"
	"log"
	"strings"

	"github.com/glitchedgitz/pocketbase/core"
	"github.com/glitchedgitz/pocketbase/daos"
)

// ProxyLabelKey normalises a proxy name for comparison. Names are unique
// ignoring case and surrounding whitespace, so "Chrome" and "chrome " count as
// the same name.
func ProxyLabelKey(label string) string {
	return strings.ToLower(strings.TrimSpace(label))
}

// proxyLabelBase strips a trailing " N" so an already-numbered name continues
// its series rather than being numbered twice: "chrome 2" alongside an existing
// "chrome" becomes "chrome 3", not "chrome 2 2". The suffix only counts as a
// series number when the stem is itself in use, which leaves a name like
// "scan 2024" alone unless a "scan" exists.
func proxyLabelBase(label string, taken map[string]bool) string {
	label = strings.TrimSpace(label)

	i := strings.LastIndex(label, " ")
	if i <= 0 {
		return label
	}

	suffix := label[i+1:]
	if suffix == "" || strings.TrimLeft(suffix, "0123456789") != "" {
		return label
	}

	stem := strings.TrimSpace(label[:i])
	if stem == "" || !taken[ProxyLabelKey(stem)] {
		return label
	}
	return stem
}

// NextProxyLabel returns desired when it is free, otherwise the lowest
// "{base} {n}" that is not taken yet. Exported so the migration that de-dupes
// pre-existing names numbers them exactly the way new proxies are numbered.
func NextProxyLabel(taken map[string]bool, desired string) string {
	desired = strings.TrimSpace(desired)
	if !taken[ProxyLabelKey(desired)] {
		return desired
	}

	base := proxyLabelBase(desired, taken)
	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s %d", base, n)
		if !taken[ProxyLabelKey(candidate)] {
			return candidate
		}
	}
}

// takenProxyLabels returns the names already in use, skipping excludeID so a
// record keeps its own name on update. Archived proxies count too: they can be
// restored, and a restore must not reintroduce a duplicate.
func takenProxyLabels(dao *daos.Dao, excludeID string) (map[string]bool, error) {
	records, err := dao.FindRecordsByExpr("_proxies")
	if err != nil {
		return nil, err
	}

	taken := make(map[string]bool, len(records))
	for _, r := range records {
		if r.GetId() == excludeID {
			continue
		}
		taken[ProxyLabelKey(r.GetString("label"))] = true
	}
	return taken, nil
}

// uniqueProxyLabel is NextProxyLabel against the names currently in the
// database. Used where the name is generated rather than typed, so a second
// Chrome proxy becomes "chrome 2" instead of a second "chrome".
func uniqueProxyLabel(dao *daos.Dao, desired string, excludeID string) string {
	taken, err := takenProxyLabels(dao, excludeID)
	if err != nil {
		log.Printf("[ProxyLabel] Could not read existing proxy names: %v", err)
		return strings.TrimSpace(desired)
	}
	return NextProxyLabel(taken, desired)
}

// SetupProxyLabelHooks rejects a rename that collides with another proxy.
// Creation numbers itself through uniqueProxyLabel, but a name the user typed
// is better refused than silently altered.
func (backend *Backend) SetupProxyLabelHooks() error {
	backend.App.OnRecordBeforeUpdateRequest("_proxies").Add(func(e *core.RecordUpdateEvent) error {
		label := strings.TrimSpace(e.Record.GetString("label"))

		// Only validate real renames: intercept toggles, color picks and
		// archive writes all land here too, and records created before this
		// rule existed may still share a name.
		if label == e.Record.OriginalCopy().GetString("label") {
			return nil
		}

		if label == "" {
			return fmt.Errorf("proxy name is required")
		}

		taken, err := takenProxyLabels(backend.App.Dao(), e.Record.Id)
		if err != nil {
			log.Printf("[ProxyLabel] Could not verify name uniqueness: %v", err)
			return nil
		}
		if taken[ProxyLabelKey(label)] {
			return fmt.Errorf("a proxy named %q already exists", label)
		}

		e.Record.Set("label", label)
		return nil
	})

	return nil
}
