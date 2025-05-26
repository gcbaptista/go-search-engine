# Git & Workflow Rules

## PR Creation Workflow

- When creating PRs, ALWAYS use the @.github/pull_request_template.md as reference
- Create a temporary file (e.g., `pr_description.md`) with the filled template content
- Use the temporary file for PR body: `gh pr create --body-file pr_description.md`
- Delete the temporary file after PR creation
- Ensure all template sections are properly filled before creating the PR

## Branch Management

- Create descriptive branch names that reflect the work being done
- Use conventional commit messages
- Keep commits focused and atomic

## Code Review

- Self-review code before creating PR
- Ensure all tests pass before requesting review
- Address all feedback before merging
