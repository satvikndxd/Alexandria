# CI definition

`github-actions-ci.yml` is the project's GitHub Actions pipeline (Go vet/test, web
typecheck/build, Postgres migration-apply check).

It lives here rather than at `.github/workflows/ci.yml` because the automated push
credentials used for this repository lack the GitHub `workflow` OAuth scope, which is
required to create or update files under `.github/workflows/`.

## To activate CI

A maintainer whose credentials have the `workflow` scope should copy this file into place:

```bash
mkdir -p .github/workflows
cp infrastructure/ci/github-actions-ci.yml .github/workflows/ci.yml
git add .github/workflows/ci.yml
git commit -m "ci: enable GitHub Actions workflow"
git push
```
