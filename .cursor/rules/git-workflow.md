# Git & Workflow Rules

## PR Creation Workflow

- When creating PRs, ALWAYS use the @.github/pull_request_template.md as reference
- **PR titles MUST follow [Conventional Commits](https://www.conventionalcommits.org/) format**
- Create a temporary file (e.g., `pr_description.md`) with the filled template content
- Use the temporary file for PR body: `gh pr create --title "feat(scope): description" --body-file pr_description.md`
- Delete the temporary file after PR creation
- Ensure all template sections are properly filled before creating the PR

## Branch Management

- Create descriptive branch names that reflect the work being done
- Use conventional commit messages following [Conventional Commits](https://www.conventionalcommits.org/)
- Keep commits focused and atomic

## Conventional Commits Standard

- **MUST** follow the [Conventional Commits specification](https://www.conventionalcommits.org/) for all commit messages and PR titles
- **Format**: `<type>[optional scope]: <description>`
- **Types**:
  - `feat:` new feature
  - `fix:` bug fix
  - `docs:` documentation changes
  - `style:` formatting, missing semicolons, etc.
  - `refactor:` code change that neither fixes a bug nor adds a feature
  - `perf:` performance improvement
  - `test:` adding missing tests
  - `chore:` updating build tasks, package manager configs, etc.
- **Examples**:
  - `feat(search): add multi-search endpoint with parallel execution`
  - `fix(indexing): resolve race condition in document addition`
  - `docs(api): update search endpoint documentation`
  - `refactor(engine): extract filtering logic into separate module`

## Code Review

- Self-review code before creating PR
- Ensure all tests pass before requesting review
- Address all feedback before merging
