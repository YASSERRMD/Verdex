# Branch Protection Policy

The `main` branch has the following protections (configure in GitHub repository settings):

- Require pull request reviews before merging: **1 approver**
- Dismiss stale pull request approvals when new commits are pushed: **enabled**
- Require status checks to pass before merging: **CI Gate**
- Require branches to be up to date before merging: **enabled**
- Do not allow bypassing the above settings: **enabled**
- Allow squash merging: **disabled**
- Allow rebase merging: **disabled**
- Allow merge commits: **enabled** (the only permitted merge strategy)
