package corpusupdater_test

import (
	"errors"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/corpusupdater"
)

func validAmendment(now time.Time) corpusupdater.Amendment {
	return corpusupdater.Amendment{
		TargetCorpus:  corpusupdater.CorpusStatute,
		TargetID:      "rule-1",
		ChangeType:    corpusupdater.ChangeTypeAmend,
		NewText:       "New rule text.",
		Citation:      "Federal Decree-Law No. 45 of 2023, Art. 12",
		EffectiveDate: now,
	}
}

func TestValidate_WellFormed(t *testing.T) {
	t.Parallel()
	now := time.Now()
	a := validAmendment(now)
	if err := corpusupdater.Validate(a, nil, now); err != nil {
		t.Errorf("Validate() on well-formed amendment = %v, want nil", err)
	}
}

func TestValidate_InvalidChangeType(t *testing.T) {
	t.Parallel()
	now := time.Now()
	a := validAmendment(now)
	a.ChangeType = corpusupdater.ChangeType("bogus")
	if err := corpusupdater.Validate(a, nil, now); !errors.Is(err, corpusupdater.ErrInvalidChangeType) {
		t.Errorf("Validate() = %v, want ErrInvalidChangeType", err)
	}
}

func TestValidate_MissingCitation(t *testing.T) {
	t.Parallel()
	now := time.Now()
	a := validAmendment(now)
	a.Citation = "   "
	if err := corpusupdater.Validate(a, nil, now); !errors.Is(err, corpusupdater.ErrMissingCitation) {
		t.Errorf("Validate() = %v, want ErrMissingCitation", err)
	}
}

func TestValidate_InvalidCorpusTarget(t *testing.T) {
	t.Parallel()
	now := time.Now()
	a := validAmendment(now)
	a.TargetCorpus = corpusupdater.CorpusTarget("bogus")
	if err := corpusupdater.Validate(a, nil, now); !errors.Is(err, corpusupdater.ErrInvalidCorpusTarget) {
		t.Errorf("Validate() = %v, want ErrInvalidCorpusTarget", err)
	}
}

func TestValidate_MissingTargetIDForAmend(t *testing.T) {
	t.Parallel()
	now := time.Now()
	a := validAmendment(now)
	a.TargetID = ""
	if err := corpusupdater.Validate(a, nil, now); !errors.Is(err, corpusupdater.ErrMissingTargetID) {
		t.Errorf("Validate() = %v, want ErrMissingTargetID", err)
	}
}

func TestValidate_MissingTargetIDForRepeal(t *testing.T) {
	t.Parallel()
	now := time.Now()
	a := validAmendment(now)
	a.ChangeType = corpusupdater.ChangeTypeRepeal
	a.TargetID = ""
	if err := corpusupdater.Validate(a, nil, now); !errors.Is(err, corpusupdater.ErrMissingTargetID) {
		t.Errorf("Validate() = %v, want ErrMissingTargetID", err)
	}
}

func TestValidate_AddDoesNotRequireTargetID(t *testing.T) {
	t.Parallel()
	now := time.Now()
	a := validAmendment(now)
	a.ChangeType = corpusupdater.ChangeTypeAdd
	a.TargetID = ""
	if err := corpusupdater.Validate(a, nil, now); err != nil {
		t.Errorf("Validate() on Add with no TargetID = %v, want nil", err)
	}
}

func TestValidate_TargetResolverRejectsUnresolvable(t *testing.T) {
	t.Parallel()
	now := time.Now()
	a := validAmendment(now)
	resolve := func(corpusupdater.CorpusTarget, string) bool { return false }
	if err := corpusupdater.Validate(a, resolve, now); !errors.Is(err, corpusupdater.ErrAmendmentNotFound) {
		t.Errorf("Validate() with unresolvable target = %v, want ErrAmendmentNotFound", err)
	}
}

func TestValidate_TargetResolverAcceptsResolvable(t *testing.T) {
	t.Parallel()
	now := time.Now()
	a := validAmendment(now)
	resolve := func(corpus corpusupdater.CorpusTarget, targetID string) bool {
		return corpus == corpusupdater.CorpusStatute && targetID == "rule-1"
	}
	if err := corpusupdater.Validate(a, resolve, now); err != nil {
		t.Errorf("Validate() with resolvable target = %v, want nil", err)
	}
}

func TestValidate_ZeroEffectiveDate(t *testing.T) {
	t.Parallel()
	now := time.Now()
	a := validAmendment(now)
	a.EffectiveDate = time.Time{}
	if err := corpusupdater.Validate(a, nil, now); !errors.Is(err, corpusupdater.ErrEffectiveDateOutOfRange) {
		t.Errorf("Validate() with zero EffectiveDate = %v, want ErrEffectiveDateOutOfRange", err)
	}
}

func TestValidate_EffectiveDateTooFarInPast(t *testing.T) {
	t.Parallel()
	now := time.Now()
	a := validAmendment(now)
	a.EffectiveDate = now.Add(-20 * 365 * 24 * time.Hour)
	if err := corpusupdater.Validate(a, nil, now); !errors.Is(err, corpusupdater.ErrEffectiveDateOutOfRange) {
		t.Errorf("Validate() with EffectiveDate 20 years ago = %v, want ErrEffectiveDateOutOfRange", err)
	}
}

func TestValidate_EffectiveDateTooFarInFuture(t *testing.T) {
	t.Parallel()
	now := time.Now()
	a := validAmendment(now)
	a.EffectiveDate = now.Add(20 * 365 * 24 * time.Hour)
	if err := corpusupdater.Validate(a, nil, now); !errors.Is(err, corpusupdater.ErrEffectiveDateOutOfRange) {
		t.Errorf("Validate() with EffectiveDate 20 years from now = %v, want ErrEffectiveDateOutOfRange", err)
	}
}

func TestValidate_EffectiveDateInFutureButWithinRange(t *testing.T) {
	t.Parallel()
	now := time.Now()
	a := validAmendment(now)
	a.EffectiveDate = now.Add(365 * 24 * time.Hour) // one year out: a real long-lead legislative date
	if err := corpusupdater.Validate(a, nil, now); err != nil {
		t.Errorf("Validate() with EffectiveDate one year out = %v, want nil", err)
	}
}
