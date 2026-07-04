# Disaster recovery runbook

This is the human-readable counterpart to `Runbook`/`RunbookStep`
(`runbook.go`) and `DefaultDRRunbook()`'s structured procedure -- the
two are kept in the same order deliberately, so this document and the
data model never drift apart silently. If you change one, change the
other in the same commit.

This runbook assumes an incident has already been detected (via
whatever alerting/on-call tooling this deployment uses) and someone is
now executing the response.

## Scope

This runbook covers scenarios where a tenant's data (or a whole
region's worth of tenants) needs to be restored from backup:

- A whole-region outage taking the primary region's storage down.
- Corruption or accidental deletion of a specific `DataClass` (case
  data, corpus/precedent, audit log, or config).
- A restore-drill-in-anger: this same procedure is what
  `Engine.RunDrill` simulates on a schedule, so "the drill" and "the
  real thing" are the same steps, exercised regularly rather than only
  discovered mid-incident.

## Prerequisites

- The on-call engineer has access to the tenant's `BackupPolicy` and
  `BackupRecord` history (via `Engine.ListBackupRecords`).
- The incident commander has confirmed which tenant(s) and
  `DataClass`(es) are affected.

## Procedure

1. **Declare the incident and page the on-call rotation.**
   Owner: on-call engineer.
   Standard incident-declaration process for this deployment; not
   specific to backup/DR.

2. **Confirm scope: which tenants and DataClasses are affected.**
   Owner: incident commander.
   Different `DataClass`es have different `BackupPolicy`s (frequency,
   retention, cross-region requirements) and different `Target`s
   (RPO/RTO) -- scope must be nailed down before a recovery point can
   be resolved.

3. **Identify the nearest recovery point per affected DataClass.**
   Owner: on-call engineer.
   Call `ResolveRecoveryPoint(records, tenantID, class, requestedAt)`
   with `requestedAt` set to the last known-good instant (usually "now,
   minus however long the incident has been silently corrupting data,
   if known" -- otherwise "now"). This returns the specific
   `BackupRecord` to restore from and its `AgeAtRequest`, which task 6
   evaluates in the next step.

4. **Verify the candidate BackupRecord's integrity before restoring
   from it.**
   Owner: on-call engineer.
   Recompute the backup artifact's hash (`ComputeIntegrityHash`) and
   call `VerifyIntegrity(record, computedHash)`. Never restore from a
   record that fails verification -- treat it as if it does not exist
   and fall back to the next-nearest recovery point instead.

5. **Restore from the verified cross-region or offline copy, per
   BackupPolicy.CrossRegionRequired.**
   Owner: on-call engineer.
   If the affected `DataClass`'s policy mandates
   `CrossRegionRequired`, and the primary region is the thing that is
   down, this is the step where that requirement pays for itself:
   restore from `LocationCrossRegion` or `LocationOffline`, not from
   the now-unavailable `LocationPrimaryRegion` copy.

6. **Run post-restore verification checks against the restored
   DataClass.**
   Owner: on-call engineer.
   Application-level sanity checks specific to the `DataClass`
   restored (e.g. case counts, audit-chain continuity via
   `auditlog.VerifyChain`-style checks) -- this package does not
   itself define those checks; they belong to the owning package for
   that data.

7. **Evaluate the restore's actual duration against the DataClass's
   RTO Target.**
   Owner: incident commander.
   Once service is restored, record how long it took (from step 1's
   declaration to now) and call `EvaluateRTO(drill, target)` with a
   `RestoreDrill` built from the real incident timeline. If `Met` is
   `false`, that is itself a finding for the post-incident review, not
   something to quietly ignore.

8. **Notify affected tenant administrators and, where applicable,
   record a compliance/breach-notification event.**
   Owner: tenant administrator liaison.
   If the incident involved actual data loss (per step 3's
   `AgeAtRequest` being non-zero) or falls under a data-protection
   breach-notification obligation, this is coordinated through
   `packages/compliance`'s `CategoryBreachNotification` controls and
   `packages/privacy`'s existing subject-notification machinery -- this
   runbook does not reinvent either.

9. **Record the incident and restore outcome as a RestoreDrill-shaped
   entry for the post-incident review.**
   Owner: incident commander.
   Whether or not this was a scheduled drill, capture it the same way
   `Engine.RunDrill` would: `Class`, `RecordID`, `ExecutedAt`,
   `Executor`, `Outcome`, `Duration`, `Notes`. A real incident and a
   scheduled drill produce the exact same shape of record, so the
   restore-drill history in `Engine.ListDrills` accumulates real
   operational proof, not just synthetic exercises.

10. **Hold a post-incident review and file follow-up actions.**
    Owner: incident commander.
    Standard postmortem process for this deployment; not specific to
    backup/DR beyond making sure any RPO/RTO miss found in steps 3/7
    becomes a tracked follow-up (e.g. tightening `BackupPolicy.Frequency`
    for the affected `DataClass`, or revisiting its `Target`).

## RPO/RTO targets this runbook is measured against

See `doc/backup-dr.md`'s RPO/RTO section for the starter
`DefaultTargets()` set per `DataClass`. A deployment may register
tighter (or looser, with sign-off) targets per tenant via
`Engine.SetTarget`.

## Restore drills exercise this same procedure

`Engine.RunDrill` (task 5) is not a separate procedure from the one
above -- it is this runbook's steps 3-7, executed on a schedule against
a real `BackupRecord`, with the outcome captured as a `RestoreDrill`
instead of (or in addition to) a live incident. Treat every drill
failure exactly as seriously as the live-incident version: if a drill
can't restore from a real backup, neither can a real incident.
