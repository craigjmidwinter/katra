package core

// Publication is the third and final transition (ingest → author → publish).
// PublishEntry is the single op both the explicit `katra stamp` and the
// automatic post-commit hook drive, so the mechanical consequences of finishing
// work — stamp the entry, close the tasks it declared, and roll up the affected
// epics — happen in exactly one place and exactly once.

// PublishResult reports what a publish did.
type PublishResult struct {
	Stamped bool
	Hashes  []string
	Closed  []string // task slugs closed (marked done + linked to the entry)
	Epics   []string // epic slugs whose rolled-up status changed
	Mutated []string // every file path written (entry + task + epic files)
}

// PublishEntry applies the deterministic consequences of finishing work: close
// the tasks the entry declares in `closes:` (marking them done and linking them
// to the entry), roll up every epic a closed task belongs to, and stamp the entry
// with its commit hash(es) + diffstat.
//
// Ordering is failure-atomic (§ fix #5): the closes + rollups (all idempotent)
// happen BEFORE the stamp, and the stamp — which is what flips the entry out of
// draft state — is applied last. So if any step fails, the entry stays a draft
// and the post-commit retry finds it again and re-runs the whole (idempotent)
// sequence to completion. Rollup errors are propagated, never silently dropped.
func (s *Store) PublishEntry(e *Entry, hashes []string) (PublishResult, error) {
	var res PublishResult

	if len(e.FM.Closes) > 0 {
		closed, err := s.CloseTasks(e.Slug, e.FM.Closes)
		if err != nil {
			return res, err
		}
		res.Closed = closed
		epics, err := s.rollupEpicsForTasks(closed)
		if err != nil {
			return res, err
		}
		res.Epics = epics
	}

	// Stamp last: this is the only step that flips the entry out of draft.
	if err := s.Stamp(e, hashes); err != nil {
		return res, err
	}
	res.Stamped = true
	res.Hashes = e.AllHashes()
	res.Mutated = s.mutatedPaths(e, res.Closed, res.Epics)
	return res, nil
}

// mutatedPaths collects the absolute file paths PublishEntry wrote — the entry
// plus every closed-task and rolled-up-epic node — so the auto-commit stages and
// commits them all as one bookkeeping commit (§ fix #6).
func (s *Store) mutatedPaths(e *Entry, closed, epics []string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		if p == "" || seen[p] {
			return
		}
		seen[p] = true
		out = append(out, p)
	}
	add(e.Path)
	for _, slug := range append(append([]string{}, closed...), epics...) {
		if n, err := s.GetNode(slug); err == nil {
			add(n.Path)
		}
	}
	return out
}

// RollupEpic recomputes an epic's status from its child tasks and writes it back
// when it changed, returning whether it changed. A rollup with no basis (no
// non-cut child tasks) leaves the stored status untouched.
func (s *Store) RollupEpic(epicSlug string) (changed bool, err error) {
	if epicSlug == "" {
		return false, nil
	}
	nodes, err := s.ListNodes()
	if err != nil {
		return false, err
	}
	want := EpicRollupStatus(nodes, epicSlug)
	if want == "" {
		return false, nil
	}
	for i := range nodes {
		e := nodes[i]
		if e.Kind() != "epic" || e.Slug != epicSlug {
			continue
		}
		if e.FM.Status == want {
			return false, nil
		}
		e.FM.Status = want
		if err := e.Save(); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

// ReconcileAdvance marks a task doing when it is still todo or specced (a spec
// existing doesn't mean work has started) and rolls up its epic. Declaring
// `advance` also records the edge on the active draft (if any) as `advances:`
// frontmatter, so the linkage survives independently of the private receipt.
// It errors if the slug is not a task.
func (s *Store) ReconcileAdvance(slug string) error {
	n, err := s.GetNode(slug)
	if err != nil {
		return err
	}
	if n.Kind() != "task" {
		return errNotATask(slug, n.Kind())
	}
	if n.FM.Status == "" || n.FM.Status == "todo" || n.FM.Status == "specced" {
		n.FM.Status = "doing"
		if err := n.Save(); err != nil {
			return err
		}
	}
	if _, err := s.RollupEpic(n.FM.Epic); err != nil {
		return err
	}
	if d, _ := s.ActiveDraft(); d != nil {
		d.FM.Advances = dedupeAppend(d.FM.Advances, slug)
		_ = d.Save()
	}
	return nil
}

// ReconcileClose records that the active draft closes a task: it appends the slug
// to the draft's `closes:` frontmatter (the closure is *declared* now and
// *applied* — done + link + epic rollup — by PublishEntry at stamp time). It
// validates the slug is a task first.
//
// An active draft is REQUIRED (§ fix #3): the closure is durably consumed only
// through the draft's `closes:` frontmatter — a receipt's close declaration is
// never consumed at publish — so declaring a close with no draft would silently
// leave the task open forever. With no draft we error, directing the agent to
// start one so the closure has somewhere durable to live.
func (s *Store) ReconcileClose(slug string) error {
	n, err := s.GetNode(slug)
	if err != nil {
		return err
	}
	if n.Kind() != "task" {
		return errNotATask(slug, n.Kind())
	}
	d, _ := s.ActiveDraft()
	if d == nil {
		return &noDraftForCloseError{slug: slug}
	}
	d.FM.Closes = dedupeAppend(d.FM.Closes, slug)
	return d.Save()
}

type noDraftForCloseError struct{ slug string }

func (e *noDraftForCloseError) Error() string {
	return "reconcile --close " + e.slug + ": no active draft to attach the closure to; " +
		"start one with `katra new \"…\"` (the task is closed when the draft is stamped)"
}

func errNotATask(slug, kind string) error {
	return &notATaskError{slug: slug, kind: kind}
}

type notATaskError struct{ slug, kind string }

func (e *notATaskError) Error() string {
	return "reconcile " + e.slug + ": not a task (it is a " + e.kind + ")"
}

// dedupeAppend appends v to list if not already present, preserving order.
func dedupeAppend(list []string, v string) []string {
	for _, x := range list {
		if x == v {
			return list
		}
	}
	return append(list, v)
}

// rollupEpicsForTasks rolls up the distinct epics owning the given tasks,
// returning the slugs whose status changed. Errors are propagated (§ fix #5) so a
// failed rollup surfaces instead of silently leaving an epic stale.
func (s *Store) rollupEpicsForTasks(taskSlugs []string) ([]string, error) {
	if len(taskSlugs) == 0 {
		return nil, nil
	}
	nodes, err := s.ListNodes()
	if err != nil {
		return nil, err
	}
	epicOf := map[string]string{}
	for i := range nodes {
		if nodes[i].Kind() == "task" {
			epicOf[nodes[i].Slug] = nodes[i].FM.Epic
		}
	}
	seen := map[string]bool{}
	var changed []string
	for _, ts := range taskSlugs {
		epic := epicOf[ts]
		if epic == "" || seen[epic] {
			continue
		}
		seen[epic] = true
		ok, err := s.RollupEpic(epic)
		if err != nil {
			return changed, err
		}
		if ok {
			changed = append(changed, epic)
		}
	}
	return changed, nil
}
