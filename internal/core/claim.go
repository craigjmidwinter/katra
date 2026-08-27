package core

import "time"

// EffectiveStatus is the status to *use* for a node, as opposed to the string
// stored in its frontmatter.
//
// For a task, `doing` is no longer written: it is derived from the presence of
// a claim. That is the whole point of the seam. "In progress" is the
// conjunction of a durable claim and a live session, and storing it means two
// systems half-knowing the same fact — so katra stores the half that survives
// (someone took this up) and leaves liveness to whoever owns now.
//
// This mirrors EpicDisplayStatus, which already derives epic status rather than
// trusting a stored cache, and for the same reason: a view cannot drift.
//
// Precedence, and the order matters:
//
//  1. A terminal stored status wins. A finished task is finished whether or not
//     someone forgot to release their claim.
//  2. A claim reads as `doing`.
//  3. Otherwise the stored status, which keeps a legacy `doing` written before
//     this existed reading as `doing`.
func (e Entry) EffectiveStatus() string {
	if e.Kind() != "task" {
		return e.FM.Status
	}
	switch e.FM.Status {
	case "done", "cut":
		return e.FM.Status
	}
	if e.IsClaimed() {
		return "doing"
	}
	return e.FM.Status
}

// IsClaimed reports whether a durable claim exists. It says nothing about
// whether the claimant is alive — katra cannot know that and must not pretend.
func (e Entry) IsClaimed() bool {
	return e.FM.ClaimedBy != ""
}

// Claim records that the current actor has taken a task up. An empty actor
// token still claims: the fact that someone took the work is worth recording
// even when the environment cannot say who, and inventing an identity here
// would be the same flattery Author refuses.
func (e *Entry) Claim(token string, at time.Time) {
	e.FM.ClaimedBy = token
	if token == "" {
		e.FM.ClaimedBy = UnknownActor
	}
	e.FM.ClaimedAt = at.UTC().Format(time.RFC3339)
}

// ReleaseClaim drops the claim, leaving the stored status untouched.
func (e *Entry) ReleaseClaim() {
	e.FM.ClaimedBy = ""
	e.FM.ClaimedAt = ""
}

// UnknownActor marks a claim made with no actor token set.
//
// Distinct from an absent claim, and the distinction is the point: no claim
// means nobody took this up, while an unknown claimant means somebody did and
// the environment could not say who. Author has no equivalent because an
// unwritten author is not a fact about the work; an unattributed claim is.
const UnknownActor = "unknown"
