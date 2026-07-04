DROP POLICY IF EXISTS tenant_isolation ON pilot_refinement_records;
ALTER TABLE pilot_refinement_records NO FORCE ROW LEVEL SECURITY;
ALTER TABLE pilot_refinement_records DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation ON pilot_findings;
ALTER TABLE pilot_findings NO FORCE ROW LEVEL SECURITY;
ALTER TABLE pilot_findings DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation ON pilot_feedback_entries;
ALTER TABLE pilot_feedback_entries NO FORCE ROW LEVEL SECURITY;
ALTER TABLE pilot_feedback_entries DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation ON pilot_cases;
ALTER TABLE pilot_cases NO FORCE ROW LEVEL SECURITY;
ALTER TABLE pilot_cases DISABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation ON pilot_deployments;
ALTER TABLE pilot_deployments NO FORCE ROW LEVEL SECURITY;
ALTER TABLE pilot_deployments DISABLE ROW LEVEL SECURITY;
